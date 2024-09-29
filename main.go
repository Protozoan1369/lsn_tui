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

type Server struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	Status   string `json:"status"`
}

type Credentials struct {
	Username string
	Password string
}

func main() {
	// Parse command line flags
	username := flag.String("username", "", "API username")
	password := flag.String("password", "", "API password")
	flag.Parse()

	// Check for required flags
	if *username == "" || *password == "" {
		fmt.Println("Please provide both username and password")
		flag.PrintDefaults()
		os.Exit(1)
	}

	credentials := Credentials{
		Username: *username,
		Password: *password,
	}

	// Create a new application
	app := tview.NewApplication()

	// Create a new table for displaying servers
	table := tview.NewTable().SetBorders(true)
	table.SetCell(0, 0, tview.NewTableCell("server_id").SetTextColor(tview.ColorYellow))
	//table.SetCell(0, 1, tview.NewTableCell("Hostname").SetTextColor(tview.ColorYellow))
	//table.SetCell(0, 2, tview.NewTableCell("Status").SetTextColor(tview.ColorYellow))

	// Fetch servers
	servers, err := fetchServers(credentials)
	if err != nil {
		log.Fatalf("Error fetching servers: %v", err)
	}

	// Populate the table with server data
	for i, server := range servers {
		table.SetCell(i+1, 0, tview.NewTableCell(server.ID))
		//table.SetCell(i+1, 1, tview.NewTableCell(server.Hostname))
		//table.SetCell(i+1, 2, tview.NewTableCell(server.Status))
	}

	// Set up key bindings
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			app.Stop()
		}
		return event
	})

	// Run the application
	if err := app.SetRoot(table, true).Run(); err != nil {
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
