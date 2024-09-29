package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/go-resty/resty/v2"
	"github.com/rivo/tview"
)

type IPSubnet struct {
	Block       string `json:"block"`
	NetworkType string `json:"network_type"`
}

type PackageItem struct {
	Category string `json:"category"`
	Option   string `json:"option"`
}

type Package struct {
	ClientID int           `json:"client_id"`
	Core     string        `json:"core"`
	Hostname string        `json:"hostname"`
	Items    []PackageItem `json:"items"`
	Name     string        `json:"name"`
	Status   string        `json:"status"`
}

type Server struct {
	ServerID     string    `json:"server_id"`
	Facility     string    `json:"facility"`
	IPSubnets    []IPSubnet `json:"ip_subnets"`
	ManagementIP string    `json:"management_ip"`
	Package      Package   `json:"package"`
}

type Credentials struct {
	Username string
	Password string
}

var table *tview.Table

func main() {
	username := flag.String("username", "", "API username")
	password := flag.String("password", "", "API password")
	flag.Parse()

	if *username == "" || *password == "" {
		fmt.Println("Please provide both username and password")
		flag.PrintDefaults()
		os.Exit(1)
	}

	credentials := Credentials{
		Username: *username,
		Password: *password,
	}

	app := tview.NewApplication()
	table = tview.NewTable().SetBorders(true)

	// Set up table headers
	headers := []string{"Server ID", "Facility", "Management IP", "Hostname", "Status", "CPU", "RAM", "Storage", "OS"}
	for i, header := range headers {
		table.SetCell(0, i, tview.NewTableCell(header).SetTextColor(tcell.ColorYellow).SetSelectable(false))
	}

	servers, err := fetchServers(credentials)
	if err != nil {
		log.Fatalf("Error fetching servers: %v", err)
	}

	for i, server := range servers {
		row := i + 1
		table.SetCell(row, 0, tview.NewTableCell(server.ServerID))
		table.SetCell(row, 1, tview.NewTableCell(server.Facility))
		table.SetCell(row, 2, tview.NewTableCell(server.ManagementIP))
		table.SetCell(row, 3, tview.NewTableCell(server.Package.Hostname))
		table.SetCell(row, 4, tview.NewTableCell(server.Package.Status))
		table.SetCell(row, 5, tview.NewTableCell(server.Package.Core))
		table.SetCell(row, 6, tview.NewTableCell(getItemOption(server.Package.Items, "RAM")))
		table.SetCell(row, 7, tview.NewTableCell(getItemOption(server.Package.Items, "Hard Drive")))
		table.SetCell(row, 8, tview.NewTableCell(getItemOption(server.Package.Items, "Operating System")))
	}

	table.Select(1, 0).SetFixed(1, 0).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			app.Stop()
		}
	}).SetSelectedFunc(func(row int, column int) {
		if row > 0 {
			showServerDetails(app, servers[row-1])
		}
	})

	if err := app.SetRoot(table, true).SetFocus(table).Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}

func fetchServers(creds Credentials) ([]Server, error) {
	client := resty.New()
	resp, err := client.R().
		SetBasicAuth(creds.Username, creds.Password).
		Get("https://api.dallas-idc.com/v1/server")
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	var servers []Server
	err = json.Unmarshal(resp.Body(), &servers)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	return servers, nil
}

func getItemOption(items []PackageItem, category string) string {
	for _, item := range items {
		if item.Category == category {
			return item.Option
		}
	}
	return "N/A"
}

func showServerDetails(app *tview.Application, server Server) {
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Server ID: %s\nFacility: %s\nManagement IP: %s\nHostname: %s\nStatus: %s\nCPU: %s\nRAM: %s\nStorage: %s\nOS: %s",
			server.ServerID,
			server.Facility,
			server.ManagementIP,
			server.Package.Hostname,
			server.Package.Status,
			server.Package.Core,
			getItemOption(server.Package.Items, "RAM"),
			getItemOption(server.Package.Items, "Hard Drive"),
			getItemOption(server.Package.Items, "Operating System"))).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			app.SetRoot(table, true)
		})

	app.SetRoot(modal, false)
}
