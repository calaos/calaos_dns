package models

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/calaos/calaos_dns/config"
	"github.com/calaos/calaos_dns/models/orm"
	"github.com/calaos/calaos_dns/utils"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/joeig/go-powerdns/v3"
	"github.com/robfig/cron"
)

var (
	db          *gorm.DB
	cronTab     *cron.Cron
	wantLogging bool
	pdns        *powerdns.Client
)

func Init(logSql bool) (err error) {
	headers := make(map[string]string)
	headers["X-API-Key"] = config.Conf.Powerdns.ApiKey
	pdns = powerdns.NewClient(config.Conf.Powerdns.Api, "localhost", headers, nil)

	wantLogging = logSql
	db, err = gorm.Open(config.Conf.Database.Type, config.Conf.Database.Dsn)
	if err != nil {
		return
	}

	err = db.DB().Ping()
	if err != nil {
		return
	}

	db.SetLogger(log.New(os.Stdout, "\n", 0))
	db.LogMode(wantLogging)
	db.DB().SetMaxIdleConns(10)
	db.DB().SetMaxOpenConns(100)
	db.DB().SetConnMaxLifetime(time.Hour)

	migrateDb()

	cronTab = cron.New()

	j := CronJob{
		Func: removeExpired,
		Name: "removeExpired()",
	}
	cronTab.AddJob("@every 2h", j)

	removeExpired()

	//start scheduler
	cronTab.Start()

	return
}

func ListCronEntries() []*cron.Entry {
	return cronTab.Entries()
}

type CronJob struct {
	cron.Job
	Func func()
	Name string
}

func (f CronJob) Run() { f.Func() }

func checkDbConnection() (err error) {
	err = db.DB().Ping()
	if err != nil {
		db.Close()
		return Init(wantLogging)
	}
	return
}

func migrateDb() {
	//Migrate all tables
	db.AutoMigrate(
		&Host{})
}

type Host struct {
	ID        int64      `gorm:"primary_key" json:"-"`
	Hostname  string     `json:"mainzone"`
	Subzones  string     `json:"subzones"`
	IP        string     `json:"ip"`
	Token     string     `json:"token,omitempty"`
	UpdatedAt *time.Time `gorm:"type:timestamp" json:"updated_at,omitempty"`
}

func removeExpired() {
	log.Println("Removing expired dns entries...")

	var hosts []Host
	err := orm.FindAll(db, &hosts)
	if err != nil {
		log.Println("Unable to query all hosts from DB:", err)
		return
	}

	tCheck := time.Now()
	tCheck = tCheck.AddDate(0, 0, 0-config.Conf.General.ExpirationDays)

	for _, h := range hosts {
		if h.UpdatedAt.Before(tCheck) {
			log.Println("Entry:", h.Hostname, "has expired.")
			deleteHost(&h)
		}
	}
}

func GetAllHosts() (hosts []Host, err error) {
	err = orm.FindAll(db, &hosts)
	if err != nil {
		log.Println("Unable to query all hosts from DB:", err)
	}
	return
}

func RegisterDns(mainzone, subzone, token, ip string) (err error, newToken string) {
	log.Println("Register new DNS:", mainzone, subzone, token, ip)
	if mainzone == "" {
		log.Println("Failure: Mainzone is empty")
		return fmt.Errorf("Mainzone is empty"), newToken
	}

	mainzone, valid := utils.IsValidHostname(mainzone)
	if !valid {
		log.Println("Failure: Invalid hostname:", mainzone)
		return fmt.Errorf("Invalid hostname"), newToken
	}

	if utils.StringInSlice(mainzone, config.Conf.Powerdns.Blacklist) {
		log.Println("Failure: Invalid hostname, is in blacklist:", mainzone)
		return fmt.Errorf("Invalid hostname"), newToken
	}

	if subzone != "" {
		subs := strings.Split(subzone, ",")
		for _, s := range subs {
			_, valid = utils.IsValidSubHostname(s)
			if !valid {
				log.Println("Failure: Invalid sub hostname:", s)
				return fmt.Errorf("Invalid hostname"), newToken
			}
		}
	}

	var h Host
	params := map[string]interface{}{
		"Hostname": mainzone,
	}
	dberr := orm.FindOneByQuery(db, &h, params)

	z := mainzone + "." + config.Conf.Powerdns.Zone

	ctx := context.Background()
	_, err = pdns.Zones.Get(ctx, config.Conf.Powerdns.Zone)
	if err != nil {
		log.Println("Unable to get zone", config.Conf.Powerdns.Zone, "from PowerDNS:", err)
		return fmt.Errorf("Internal error"), newToken
	}

	if token == "" { //User wants to register a subdomain
		if dberr == nil { //but this host already exists
			log.Println("Host", mainzone, "already exists in DB")
			return fmt.Errorf("Host already registered"), newToken
		}

		h.Hostname = mainzone
		h.Subzones = subzone
		h.IP = ip
		h.Token = utils.TokenGenerator()

		log.Println("Adding new host to DB with token:", h.Token)

		err = orm.Create(db, &h)
		if err != nil {
			log.Println("Failed to add entry to DB:", err)
			return fmt.Errorf("Internal error"), newToken
		}

		log.Println("Adding record to PowerDNS:", z)

		err = pdns.Records.Add(ctx, config.Conf.Powerdns.Zone, z, powerdns.RRTypeA, 60, []string{ip})
		if err != nil {
			log.Println("Unable to add mainzone", z, ":", err)

			//Something went wrong, delete everything for this host
			deleteHost(&h)

			return fmt.Errorf("Internal error"), newToken
		}

		if subzone != "" {
			subs := strings.Split(subzone, ",")
			for _, s := range subs {
				sz := s + "." + z
				log.Println("Adding record to PowerDNS:", sz)
				err = pdns.Records.Add(ctx, config.Conf.Powerdns.Zone, sz, powerdns.RRTypeA, 60, []string{ip})
				if err != nil {
					log.Println("Unable to add subzone", sz, ":", err)

					//Something went wrong, delete everything for this host
					deleteHost(&h)

					return fmt.Errorf("Internal error"), newToken
				}
			}
		}
	} else { //User has passed his token, do an update

		//Check if his token is the right one
		if h.Token != token {
			return fmt.Errorf("Wrong token"), newToken
		}

		if h.Subzones != subzone {
			if h.Subzones != "" {
				subs := strings.Split(h.Subzones, ",")
				for _, s := range subs {
					sz := s + "." + z
					log.Println("Deleting record from PowerDNS:", sz)
					err = pdns.Records.Delete(ctx, config.Conf.Powerdns.Zone, sz, powerdns.RRTypeA)
					if err != nil {
						log.Println("Unable to delete subzone", sz, ":", err)
					}
				}
			}

			if subzone != "" {
				subs := strings.Split(subzone, ",")
				for _, s := range subs {
					sz := s + "." + z
					log.Println("Adding record to PowerDNS:", sz)
					err = pdns.Records.Add(ctx, config.Conf.Powerdns.Zone, sz, powerdns.RRTypeA, 60, []string{ip})
					if err != nil {
						log.Println("Unable to add subzone", sz, ":", err)

						//Something went wrong, delete everything for this host
						deleteHost(&h)

						return fmt.Errorf("Internal error"), newToken
					}
				}
			}

			h.Subzones = subzone
		}

		if h.IP != ip {
			log.Println("Updating record from PowerDNS:", z)
			err = pdns.Records.Change(ctx, config.Conf.Powerdns.Zone, z, powerdns.RRTypeA, 60, []string{ip})
			if err != nil {
				log.Println("Unable to update mainzone", z, ":", err)
			}

			h.IP = ip
		}

		err = orm.Save(db, &h)
		if err != nil {
			log.Println("Faild to save to db:", err)
		}
	}

	return nil, h.Token
}

func DeleteDns(token string) (err error) {
	log.Println("Deleting host for token:", token)

	var h Host
	params := map[string]interface{}{
		"Token": token,
	}
	err = orm.FindOneByQuery(db, &h, params)
	if err != nil {
		log.Println("Token has not been found:", err)
		return fmt.Errorf("Unknown token")
	}

	return deleteHost(&h)
}

func deleteHost(h *Host) (err error) {

	ctx := context.Background()
	zone, err := pdns.Zones.Get(ctx, config.Conf.Powerdns.Zone)
	if err != nil {
		log.Println("Unable to get zone", config.Conf.Powerdns.Zone, "from PowerDNS:", err)
		return
	}

	z := h.Hostname + "." + config.Conf.Powerdns.Zone

	//list of possible _acme-challenge.*** records
	acme := []string{
		"_acme-challenge." + z,
	}

	if h.Subzones != "" {
		subs := strings.Split(h.Subzones, ",")
		for _, s := range subs {
			acme = append(acme, "_acme-challenge."+s+"."+z)
		}
	}

	//Delete all _acme-challenge.*** if any. They are used for letsencrypt
	for _, rr := range zone.RRsets {
		if utils.StringInSlice(strings.Trim(*rr.Name, "."), acme) {
			log.Println("Deleting record from PowerDNS:", rr.Name)
			err = pdns.Records.Delete(ctx, config.Conf.Powerdns.Zone, *rr.Name, powerdns.RRTypeTXT)
			if err != nil {
				log.Println("Unable to delete record", rr.Name, ":", err)
			}
		}
	}

	if h.Subzones != "" {
		subs := strings.Split(h.Subzones, ",")
		for _, s := range subs {
			sz := s + "." + z
			log.Println("Deleting record from PowerDNS:", sz)
			err = pdns.Records.Delete(ctx, config.Conf.Powerdns.Zone, sz, powerdns.RRTypeA)
			if err != nil {
				log.Println("Unable to delete subzone", sz, ":", err)
			}
		}
	}

	log.Println("Deleting record from PowerDNS:", z)
	err = pdns.Records.Delete(ctx, config.Conf.Powerdns.Zone, z, powerdns.RRTypeA)
	if err != nil {
		log.Println("Unable to delete mainzone", z, ":", err)
	}

	err = orm.Delete(db, &h)
	if err != nil {
		log.Println("Unable to delete zone in DB:", err)
	}

	return err
}

func UpdateDns(token, ip string) (err error) {
	log.Println("Updating IP for token:", token, ip)

	var h Host
	params := map[string]interface{}{
		"Token": token,
	}
	err = orm.FindOneByQuery(db, &h, params)
	if err != nil {
		log.Println("Token has not been found:", err)
		return fmt.Errorf("Unknown token")
	}

	ctx := context.Background()
	_, err = pdns.Zones.Get(ctx, config.Conf.Powerdns.Zone)
	if err != nil {
		log.Println("Unable to get zone", config.Conf.Powerdns.Zone, "from PowerDNS:", err)
	}

	z := h.Hostname + "." + config.Conf.Powerdns.Zone

	if h.IP != ip {
		log.Println("Updating record from PowerDNS:", z)
		err = pdns.Records.Change(ctx, config.Conf.Powerdns.Zone, z, powerdns.RRTypeA, 60, []string{ip})
		if err != nil {
			log.Println("Unable to update mainzone", z, ":", err)
		}

		if h.Subzones != "" {
			subs := strings.Split(h.Subzones, ",")
			for _, s := range subs {
				sz := s + "." + z
				log.Println("Updating record from PowerDNS:", sz)
				err = pdns.Records.Change(ctx, config.Conf.Powerdns.Zone, sz, powerdns.RRTypeA, 60, []string{ip})
				if err != nil {
					log.Println("Unable to update subzone", sz, ":", err)
				}
			}
		}

		h.IP = ip
	}

	err = orm.Save(db, &h)
	if err != nil {
		log.Println("Faild to save to db:", err)
	}

	return nil
}

func AddLeRecord(token, leDomain, leToken string) (err error) {
	log.Println("Add Letsencrypt token for user", token, ". Domain:", leDomain, "Token:", leToken)

	var h Host
	params := map[string]interface{}{
		"Token": token,
	}
	err = orm.FindOneByQuery(db, &h, params)
	if err != nil {
		log.Println("Token has not been found:", err)
		return fmt.Errorf("Unknown token")
	}

	if leDomain == "" || leToken == "" {
		log.Println("Emtpy domain/token:", leDomain, ",", leToken)
		return fmt.Errorf("Bad input")
	}

	ctx := context.Background()
	_, err = pdns.Zones.Get(ctx, config.Conf.Powerdns.Zone)
	if err != nil {
		log.Println("Unable to get zone", config.Conf.Powerdns.Zone, "from PowerDNS:", err)
	}

	//Check if domain is registered for this host
	subs := strings.Split(h.Subzones, ",")
	if leDomain != h.Hostname && !utils.StringInSlice(leDomain, subs) {
		log.Println("Wrong domain, not registered for user")
		return fmt.Errorf("Wrong domain")
	}

	z := leDomain + "." + config.Conf.Powerdns.Zone
	if leDomain != h.Hostname { //It's a subdomain
		z = leDomain + "." + h.Hostname + "." + config.Conf.Powerdns.Zone
	}
	acme := "_acme-challenge." + z

	err = pdns.Records.Add(ctx, config.Conf.Powerdns.Zone, acme, powerdns.RRTypeTXT, 60, []string{"\"" + leToken + "\""})
	if err != nil {
		log.Println("Unable to update zone for letsencrypt", acme, ":", err)
	}

	return
}

func DeleteLeRecord(token, leDomain string) (err error) {
	log.Println("Delete Letsencrypt token for user", token, ". Domain:", leDomain)

	var h Host
	params := map[string]interface{}{
		"Token": token,
	}
	err = orm.FindOneByQuery(db, &h, params)
	if err != nil {
		log.Println("Token has not been found:", err)
		return fmt.Errorf("Unknown token")
	}

	if leDomain == "" {
		log.Println("Emtpy domain:", leDomain)
		return fmt.Errorf("Bad input")
	}

	ctx := context.Background()
	_, err = pdns.Zones.Get(ctx, config.Conf.Powerdns.Zone)
	if err != nil {
		log.Println("Unable to get zone", config.Conf.Powerdns.Zone, "from PowerDNS:", err)
	}

	//Check if domain is registered for this host
	subs := strings.Split(h.Subzones, ",")
	if leDomain != h.Hostname && !utils.StringInSlice(leDomain, subs) {
		log.Println("Wrong domain, not registered for user")
		return fmt.Errorf("Wrong domain")
	}

	z := leDomain + "." + config.Conf.Powerdns.Zone
	if leDomain != h.Hostname { //It's a subdomain
		z = leDomain + "." + h.Hostname + "." + config.Conf.Powerdns.Zone
	}
	acme := "_acme-challenge." + z

	err = pdns.Records.Delete(ctx, config.Conf.Powerdns.Zone, acme, powerdns.RRTypeTXT)
	if err != nil {
		log.Println("Unable to update zone for letsencrypt", acme, ":", err)
	}

	return
}

func GetPdnsRecords(h *Host) (records []string) {
	ctx := context.Background()
	zone, err := pdns.Zones.Get(ctx, config.Conf.Powerdns.Zone)
	if err != nil {
		log.Println("Unable to get zone", config.Conf.Powerdns.Zone, "from PowerDNS:", err)
		return
	}

	z := h.Hostname + "." + config.Conf.Powerdns.Zone

	//list of possible _acme-challenge.*** records
	acme := []string{
		"_acme-challenge." + z,
	}

	if h.Subzones != "" {
		subs := strings.Split(h.Subzones, ",")
		for _, s := range subs {
			acme = append(acme, "_acme-challenge."+s+"."+z)
			acme = append(acme, s+"."+z)
		}
	}

	//Delete all _acme-challenge.*** if any. They are used for letsencrypt
	for _, rr := range zone.RRsets {
		if utils.StringInSlice(strings.Trim(*rr.Name, "."), acme) ||
			strings.Trim(*rr.Name, ".") == z {
			records = append(records, formatRecord(&rr))
		}
	}

	return
}

func formatRecord(rr *powerdns.RRset) (s string) {
	content := ""
	if len(rr.Records) > 0 {
		content = *rr.Records[0].Content
	}
	s = fmt.Sprintf("%v\t%v\t%v\t%v", *rr.Name, *rr.Type, *rr.TTL, content)
	return
}
