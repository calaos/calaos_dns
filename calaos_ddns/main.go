package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/calaos/calaos_dns/calaos_ddns/calaos"
	lecert "github.com/calaos/calaos_dns/calaos_ddns/cert"
	"github.com/calaos/calaos_dns/calaos_ddns/haproxy"
	"github.com/calaos/calaos_dns/utils"
	"github.com/dghubble/sling"
	"github.com/fatih/color"
	"github.com/jawher/mow.cli"
	"github.com/mattn/go-isatty"
	"github.com/xenolf/lego/platform/config/env"
)

const (
	CharStar     = "\u2737"
	CharAbort    = "\u2718"
	CharCheck    = "\u2714"
	CharWarning  = "\u26A0"
	CharArrow    = "\u2012\u25b6"
	CharVertLine = "\u2502"

	CALAOS_NS    = "https://ns1.calaos.fr/"
	KEY_TOKEN    = "ddns_token"
	KEY_LE_EMAIL = "ddns_le_email"
)

var (
	blue       = color.New(color.FgBlue).SprintFunc()
	errorRed   = color.New(color.FgRed).SprintFunc()
	errorBgRed = color.New(color.BgRed, color.FgBlack).SprintFunc()
	green      = color.New(color.FgGreen).SprintFunc()
	cyan       = color.New(color.FgCyan).SprintFunc()
	bgCyan     = color.New(color.FgWhite).SprintFunc()

	csling *sling.Sling
)

var (
	conffile *string
)

func exit(err error, exit int) {
	fmt.Fprintln(os.Stderr, errorRed(CharAbort), err)
	cli.Exit(exit)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	mnApp := cli.App("calaos_ddns", "Client for Calaos Dynamic DNS API")

	mnApp.Command("register", "Register a DNS record", cmdRegister)
	mnApp.Command("unregister", "Unregister the saved DNS record", cmdUnregister)
	mnApp.Command("update", "Update IP of DNS record", cmdUpdate)
	mnApp.Command("reset", "Reset an expired token", cmdReset)

	if _, _, err := utils.InitLogger(); err != nil {
		exit(err, 1)
	}

	if err := mnApp.Run(os.Args); err != nil {
		exit(err, 1)
	}
}

type RegisterJson struct {
	Mainzone string `json:"mainzone"`
	Subzones string `json:"subzones"`
	Token    string `json:"token"`
}

func cmdRegister(cmd *cli.Cmd) {
	cmd.Spec = "DOMAIN [SUBDOMAIN...]"
	var (
		domain     = cmd.StringArg("DOMAIN", "", "The subdomain to register (ex. myname would register myname.calaos.fr)")
		subdomains = cmd.StringsArg("SUBDOMAIN", nil, "The subdomains to register under your maindomain (ex. camera1 would register camera1.myname.calaos.fr) camera1=192.168.0.1:4444 will also create a haproxy entry for pointing to the correct IP address")
	)

	cmd.Action = func() {
		err, token := calaos.GetConfig(KEY_TOKEN)
		if err != nil {
			exit(fmt.Errorf("Error reading calaos config:", err), 1)
			return
		}

		haconf, err := haproxy.ParseDomains(*domain, *subdomains)
		if err != nil {
			exit(fmt.Errorf("Error parsing domain redirections: %v", err), 1)
			return
		}

		mainDomain := ""
		subzones := []string{}
		for i, b := range haconf.Backends {
			if i == 0 {
				mainDomain = haconf.Backends[0].Name
			} else {
				subzones = append(subzones, b.Name)
			}
		}

		jdata := RegisterJson{
			Mainzone: mainDomain,
			Subzones: strings.Join(subzones, ","),
			Token:    token,
		}

		slingReq, err := sling.New().Base(CALAOS_NS).Post("api/register").BodyJSON(jdata).Request()
		if err != nil {
			exit(fmt.Errorf("Error creating sling:", err), 1)
			return
		}

		data, err := doRequest(slingReq)
		if err != nil {
			fmt.Println(err)
		} else {
			r := RegisterJson{}
			if err := json.Unmarshal(data, &r); err != nil {
				exit(fmt.Errorf("Failed to unmarshal data: %v, Error: %v\n", data, err), 1)
				return
			}

			//Save token before trying to get LE cert. In case of failure user can then unregister the domain
			err = calaos.SetConfig(KEY_TOKEN, r.Token)
			if err != nil {
				exit(fmt.Errorf("Failed to save token:", err), 1)
				return
			}

			err, le_email := calaos.GetConfig(KEY_LE_EMAIL)
			if err != nil {
				exit(fmt.Errorf("Error reading calaos config:", err), 1)
				return
			}

			if le_email == "" {
				fmt.Println(CharWarning, "To generate a Let's Encrypt certificate, you need to set a user email address.")
				fmt.Print("Enter your email address: ")
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()

				email := scanner.Text()

				if email == "" {
					fmt.Println("An emtpy email address is not valid. No certificate will be generated.")
				} else {
					le_email = email
					err = calaos.SetConfig(KEY_LE_EMAIL, strings.TrimSpace(le_email))
					if err != nil {
						fmt.Println("Failed to save le_email:", err)
					}
				}
			}

			if len(le_email) != 0 {
				//Get a certificate from let's encrypt
				var le_domain []string
				le_domain = append(le_domain, mainDomain+".calaos.fr")
				for i, b := range haconf.Backends {
					if i == 0 {
						continue
					}
					d := fmt.Sprintf("%s.%s.calaos.fr", b.Name, mainDomain)
					le_domain = append(le_domain, d)
				}

				var s *spinner.Spinner
				if isatty.IsTerminal(os.Stdout.Fd()) {
					s = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
					s.Suffix = "  Getting a certificate from Let's Encrypt. Please wait..."
					s.Color("blue")
					s.Start()
				}

				err = lecert.GenerateCert(le_domain, le_email)
				if isatty.IsTerminal(os.Stdout.Fd()) {
					s.Stop()
				}
				if err != nil {
					fmt.Println(errorRed(CharAbort), "Failed to generate certificate:", err)
				} else {

					//Copy the certificate to haproxy location and concatenate key+cert
					err = lecert.WritePemFile(env.GetOrDefaultString("CALAOS_CERT_FILE", "/etc/ssl/haproxy/server.pem"))
					if err != nil {
						fmt.Println(errorRed(CharAbort), "Failed to write certificate:", err)
					}

					templateFile := filepath.Join(env.GetOrDefaultString("CALAOS_HAPROXY_PATH", "/etc/haproxy"), "haproxy.template")
					outFile := filepath.Join(env.GetOrDefaultString("CALAOS_HAPROXY_PATH", "/etc/haproxy"), "haproxy.cfg")
					err = haproxy.RenderConfig(outFile, templateFile, haconf)
					if err != nil {
						fmt.Println(errorRed(CharAbort), "Failed to write haproxy config:", err)
					}
				}
			}

			if err == nil {
				color.Green(CharCheck + " Register successful.")
			}
		}
	}
}

func cmdUnregister(cmd *cli.Cmd) {
	cmd.Action = func() {
		err, token := calaos.GetConfig(KEY_TOKEN)
		if err != nil {
			exit(fmt.Errorf("Error reading calaos config:", err), 1)
			return
		}

		if token == "" {
			exit(fmt.Errorf("Saved token not found. Nothing to unregister."), 1)
			return
		}

		slingReq, err := sling.New().Base(CALAOS_NS).Delete("api/delete/" + token).Request()
		if err != nil {
			exit(fmt.Errorf("Error creating sling:", err), 1)
			return
		}

		_, err = doRequest(slingReq)
		if err != nil {
			fmt.Println(err)
		} else {
			err = calaos.DeleteConfig(KEY_TOKEN)
			if err != nil {
				exit(fmt.Errorf("Failed to delete token from config:", err), 1)
				return
			}

			color.Green(CharCheck + " Unregister successful.")
		}
	}
}

func cmdReset(cmd *cli.Cmd) {
	cmd.Action = func() {
		err := calaos.DeleteConfig(KEY_TOKEN)
		if err != nil {
			exit(fmt.Errorf("Failed to reset token from config:", err), 1)
			return
		}

		color.Green(CharCheck + " Reset successful.")
	}
}

func cmdUpdate(cmd *cli.Cmd) {
	cmd.Spec = "[-f]"
	var (
		forceRenew = cmd.BoolOpt("f force-renew", false, "Force renew even if certificate is not expired")
	)

	cmd.Action = func() {
		err, token := calaos.GetConfig(KEY_TOKEN)
		if err != nil {
			exit(fmt.Errorf("Error reading calaos config:", err), 1)
			return
		}

		if token == "" {
			exit(fmt.Errorf("Saved token not found. Nothing to update."), 0)
			return
		}

		slingReq, err := sling.New().Base(CALAOS_NS).Get("api/update/" + token).Request()
		if err != nil {
			exit(fmt.Errorf("Error creating sling:", err), 1)
			return
		}

		_, err = doRequest(slingReq)
		if err != nil {
			exit(err, 1)
		}

		var s *spinner.Spinner
		if isatty.IsTerminal(os.Stdout.Fd()) {
			s = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
			s.Suffix = "  Renewing certificate. Please wait..."
			s.Color("blue")
			s.Start()
		}

		hasrenew, err := lecert.MaybeRenew(*forceRenew)
		if isatty.IsTerminal(os.Stdout.Fd()) {
			s.Stop()
		}
		if err != nil {
			exit(err, 1)
		}

		if err == nil {
			if hasrenew {
				//Copy the certificate to haproxy location and concatenate key+cert
				err = lecert.WritePemFile(env.GetOrDefaultString("CALAOS_CERT_FILE", "/etc/ssl/haproxy/server.pem"))
				if err != nil {
					fmt.Println(errorRed(CharAbort), "Failed to write certificate:", err)
				}

				//Restart haproxy
				cmd := exec.Command("/bin/systemctl", "restart", "haproxy.service")
				err := cmd.Run()
				if err != nil {
					fmt.Println(errorRed(CharAbort), "Failed to restart haproxy... ", err)
				}

				color.Green(CharCheck + " Update successful. Certificate has been renewed")
			} else {
				color.Green(CharCheck + " Update successful.")
			}
		}
	}
}

func doRequest(req *http.Request) (data []byte, err error) {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return doRequestReal(req)
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = "  please wait..."
	s.Color("blue")
	s.Start()
	defer s.Stop()

	return doRequestReal(req)
}

func doRequestReal(req *http.Request) (data []byte, err error) {
	failJson := &struct {
		Message string `json:"message"`
	}{Message: ""}

	client := &http.Client{}
	if resp, err := client.Do(req); err != nil {
		fmt.Println("Error:", err)
		return nil, err
	} else {
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			json.NewDecoder(resp.Body).Decode(&failJson)
			return nil, fmt.Errorf("Error: %v", failJson.Message)
		}
		return ioutil.ReadAll(resp.Body)
	}

	return
}
