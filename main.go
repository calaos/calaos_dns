package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/calaos/calaos_dns/app"
	"github.com/calaos/calaos_dns/config"
	"github.com/calaos/calaos_dns/models"

	"github.com/fatih/color"
	"github.com/jawher/mow.cli"
)

const (
	CONFIG_FILENAME = "/etc/calaos_dns.conf"

	CharStar     = "\u2737"
	CharAbort    = "\u2718"
	CharCheck    = "\u2714"
	CharWarning  = "\u26A0"
	CharArrow    = "\u2012\u25b6"
	CharVertLine = "\u2502"
)

var (
	blue       = color.New(color.FgBlue).SprintFunc()
	errorRed   = color.New(color.FgRed).SprintFunc()
	errorBgRed = color.New(color.BgRed, color.FgBlack).SprintFunc()
	green      = color.New(color.FgGreen).SprintFunc()
	cyan       = color.New(color.FgCyan).SprintFunc()
	bgCyan     = color.New(color.FgWhite).SprintFunc()
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

	mnApp := cli.App("calaos_dns", "Backend for Calaos Dynamic DNS API endpoint")

	mnApp.Spec = "[-c]"

	conffile = mnApp.StringOpt("c config", CONFIG_FILENAME, "Set config file")

	mnApp.Command("zone", "DNS zones management", func(cmd *cli.Cmd) {
		cmd.Command("list", "list all registered subdomains", cmdDnsList)
		cmd.Command("delete", "delete a registered subdomains", cmdDnsDelete)
	})

	//Main action of the tool is to start the webserver
	mnApp.Action = func() {
		if err := app.Init(conffile); err != nil {
			exit(err, 1)
		}

		if err := models.Init(true); err != nil {
			exit(err, 1)
		}

		if err := app.Run(); err != nil {
			exit(err, 1)
		}
	}

	if err := mnApp.Run(os.Args); err != nil {
		exit(err, 1)
	}
}

func cmdDnsList(cmd *cli.Cmd) {
	cmd.Action = func() {
		if err := config.ReadConfig(*conffile); err != nil {
			fmt.Printf("Failed to read config file: %v", err)
			return
		}

		if err := models.Init(false); err != nil {
			exit(err, 1)
		}

		hosts, err := models.GetAllHosts()
		if err != nil {
			fmt.Println("failed to get hosts", err)
			return
		}

		fmt.Printf("Hosts:\n")
		fmt.Printf("---------------------\n")
		for _, h := range hosts {
			fmt.Printf("[%v] - %v\n", h.ID, h.Hostname+"."+config.Conf.Powerdns.Zone)
			fmt.Printf("\tIP:\t\t%v\n", h.IP)
			fmt.Printf("\tToken:\t\t%v\n", h.Token)
			fmt.Printf("\tSubdomains:\t%v\n", h.Subzones)
			tCheck := time.Now()
			tCheck = tCheck.AddDate(0, 0, 0-config.Conf.General.ExpirationDays)
			d := h.UpdatedAt.Sub(tCheck)
			fmt.Printf("\tExpires in:\t%v\n", d)
		}
	}

}

func cmdDnsDelete(cmd *cli.Cmd) {
	cmd.Spec = "TOKEN"
	var (
		token = cmd.StringArg("TOKEN", "", "Token for the zone")
	)

	cmd.Action = func() {
		if err := config.ReadConfig(*conffile); err != nil {
			fmt.Printf("Failed to read config file: %v", err)
			return
		}

		if err := models.Init(false); err != nil {
			exit(err, 1)
		}

		err := models.DeleteDns(*token)
		if err != nil {
			fmt.Println("failed to delete host:", err)
		} else {
			fmt.Println("Host deleted")
		}
	}

}
