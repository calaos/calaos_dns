package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/calaos/calaos_dns/calaos_ddns/calaos"
	"github.com/dghubble/sling"
	"github.com/fatih/color"
	"github.com/jawher/mow.cli"
	"github.com/mattn/go-isatty"
)

const (
	CharStar     = "\u2737"
	CharAbort    = "\u2718"
	CharCheck    = "\u2714"
	CharWarning  = "\u26A0"
	CharArrow    = "\u2012\u25b6"
	CharVertLine = "\u2502"

	CALAOS_NS = "https://ns1.calaos.fr/"
	KEY_TOKEN = "ddns_token"
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
		subdomains = cmd.StringsArg("SUBDOMAIN", nil, "The subdomains to register under your maindomain (ex. camera1 would register camera1.myname.calaos.fr)")
	)

	cmd.Action = func() {
		err, token := calaos.GetConfig(KEY_TOKEN)
		if err != nil {
			fmt.Println("Error reading calaos config:", err)
			return
		}

		jdata := RegisterJson{
			Mainzone: *domain,
			Subzones: strings.Join(*subdomains, ","),
			Token:    token,
		}

		slingReq, err := sling.New().Base(CALAOS_NS).Post("api/register").BodyJSON(jdata).Request()
		if err != nil {
			fmt.Println("Error creating sling:", err)
			return
		}

		data, err := doRequest(slingReq)
		if err != nil {
			fmt.Println(err)
		} else {
			r := RegisterJson{}
			if err := json.Unmarshal(data, &r); err != nil {
				fmt.Printf("Failed to unmarshal data: %v, Error: %v\n", data, err)
				return
			}

			err = calaos.SetConfig(KEY_TOKEN, r.Token)
			if err != nil {
				fmt.Println("Failed to save token:", err)
				return
			}

			fmt.Println("Register successful.")
		}
	}
}

func cmdUnregister(cmd *cli.Cmd) {
	cmd.Action = func() {
		err, token := calaos.GetConfig(KEY_TOKEN)
		if err != nil {
			fmt.Println("Error reading calaos config:", err)
			return
		}

		if token == "" {
			fmt.Println("Saved token not found. Nothing to unregister.")
			return
		}

		slingReq, err := sling.New().Base(CALAOS_NS).Delete("api/delete/" + token).Request()
		if err != nil {
			fmt.Println("Error creating sling:", err)
			return
		}

		_, err = doRequest(slingReq)
		if err != nil {
			fmt.Println(err)
		} else {
			err = calaos.DeleteConfig(KEY_TOKEN)
			if err != nil {
				fmt.Println("Failed to delete token from config:", err)
				return
			}
			fmt.Println("Unregister successful.")
		}
	}
}

func cmdUpdate(cmd *cli.Cmd) {
	cmd.Action = func() {
		err, token := calaos.GetConfig(KEY_TOKEN)
		if err != nil {
			fmt.Println("Error reading calaos config:", err)
			return
		}

		if token == "" {
			fmt.Println("Saved token not found. Nothing to update.")
			return
		}

		slingReq, err := sling.New().Base(CALAOS_NS).Get("api/update/" + token).Request()
		if err != nil {
			fmt.Println("Error creating sling:", err)
			return
		}

		_, err = doRequest(slingReq)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("Update successful.")
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
