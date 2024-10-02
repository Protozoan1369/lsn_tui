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

type RebootResponse struct {
	Status bool  `json:"status"`
	Message string `json:"message"`
}

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

var (
	app         *tview.Application
	pages       *tview.Pages
	serverTable *tview.Table
	credentials Credentials
	servers     []Server
	apiUrl = "https://api.dallas-idc.com/v1/server"
)

func main() {
	username := flag.String("username", "", "API username")
	password := flag.String("password", "", "API password")
	flag.Parse()

	if *username == "" || *password == "" {
		fmt.Println("Please provide both username and password")
		flag.PrintDefaults()
		os.Exit(1)
	}

	credentials = Credentials{
		Username: *username,
		Password: *password,
	}

	app = tview.NewApplication()
	pages = tview.NewPages()

	if err := fetchServers(); err != nil {
		log.Fatalf("Error fetching servers: %v", err)
	}

	showServerList()

	if err := app.SetRoot(pages, true).EnableMouse(true).Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}

func showServerList() {
	// Create a flex layout
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	// Create a new flex for the title and total count
	topFlex := tview.NewFlex().SetDirection(tview.FlexColumn)

	// Add title to the left
	title := tview.NewTextView().SetText("Server List").SetTextColor(tcell.ColorWhite)
	topFlex.AddItem(title, 0, 1, false)

	// Add total count to the right
	totalCount := tview.NewTextView().SetText(fmt.Sprintf("Total Servers: %d", len(servers))).SetTextColor(tcell.ColorGreen).SetTextAlign(tview.AlignRight)
	topFlex.AddItem(totalCount, 0, 1, false)

	// Add the top flex to the main flex
	flex.AddItem(topFlex, 1, 0, false)

	// Create and populate the server table
	serverTable = tview.NewTable().SetSelectable(true, false)

	// Add headers
	serverTable.SetCell(0, 0, tview.NewTableCell("ID").SetTextColor(tcell.ColorYellow).SetSelectable(false))
	serverTable.SetCell(0, 1, tview.NewTableCell("name").SetTextColor(tcell.ColorYellow).SetSelectable(false))
	serverTable.SetCell(0, 2, tview.NewTableCell("Public IP").SetTextColor(tcell.ColorYellow).SetSelectable(false))

	// Add server data
	for i, server := range servers {
		publicIP := getPublicIP(server.IPSubnets)
		serverTable.SetCell(i+1, 0, tview.NewTableCell(server.ServerID))
		serverTable.SetCell(i+1, 1, tview.NewTableCell(server.Package.Name))
		serverTable.SetCell(i+1, 2, tview.NewTableCell(publicIP))
	}

	serverTable.Select(1, 0).SetFixed(1, 0).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			app.Stop()
		}
	}).SetSelectedFunc(func(row int, column int) {
		if row > 0 && row <= len(servers) {
			showServerMenu(servers[row-1])
		}
	})

	// Add the server table to the main flex
	flex.AddItem(serverTable, 0, 1, true)

	// Add the flex layout to the pages
	pages.AddPage("serverList", flex, true, true)
}

func showServerMenu(server Server) {
	menu := tview.NewList().ShowSecondaryText(false)
	menu.SetBorder(true).SetTitle(fmt.Sprintf("Server: %s", server.ServerID))

	menu.AddItem("View Details", "", 'd', func() {
		showServerDetails(server)
	})
    menu.AddItem("Restart Server", "", 'r', func() {
        showConfirmationDialog(server, func() {
            statusCode, rebootResp, err := rebootServer(server.ServerID)
            if err != nil {
                showMessage(fmt.Sprintf("Error restarting server %s: %v", server.ServerID, err))
            } else {
                message := fmt.Sprintf("Restart command for server %s\nStatus Code: %d\nSuccess: %t\nMessage: %s",
                    server.ServerID, statusCode, rebootResp.Status, rebootResp.Message)
                showMessage(message)
            }
        })
    })
	menu.AddItem("Power Off Server", "", 'o', func() {
		powerOffServer(server.ServerID)
	})
	menu.AddItem("Power On Server", "", 'n', func() {
		powerOnServer(server.ServerID)
	})
	menu.AddItem("Back to Server List", "", 'b', func() {
		pages.SwitchToPage("serverList")
	})

	pages.AddPage("serverMenu", menu, true, true)
	pages.SwitchToPage("serverMenu")
}

func showServerDetails(server Server) {
	details := tview.NewTextView().SetDynamicColors(true).SetRegions(true)
	details.SetBorder(true).SetTitle(fmt.Sprintf("Details: %s", server.ServerID))

	fmt.Fprintf(details, "[yellow]Server ID:[white] %s\n", server.ServerID)
	fmt.Fprintf(details, "[yellow]Hostname:[white] %s\n", server.Package.Hostname)
	fmt.Fprintf(details, "[yellow]Facility:[white] %s\n", server.Facility)
	fmt.Fprintf(details, "[yellow]Management IP:[white] %s\n", server.ManagementIP)
	fmt.Fprintf(details, "[yellow]Status:[white] %s\n", server.Package.Status)
	fmt.Fprintf(details, "[yellow]CPU:[white] %s\n", server.Package.Core)
	fmt.Fprintf(details, "[yellow]RAM:[white] %s\n", getItemOption(server.Package.Items, "RAM"))
	fmt.Fprintf(details, "[yellow]Storage:[white] %s\n", getItemOption(server.Package.Items, "Hard Drive"))
	fmt.Fprintf(details, "[yellow]OS:[white] %s\n", getItemOption(server.Package.Items, "Operating System"))

	details.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			pages.SwitchToPage("serverMenu")
		}
	})

	pages.AddPage("serverDetails", details, true, true)
	pages.SwitchToPage("serverDetails")
}

func fetchServers() error {
	client := resty.New()
	resp, err := client.R().
		SetBasicAuth(credentials.Username, credentials.Password).
		Get(apiUrl)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	err = json.Unmarshal(resp.Body(), &servers)
	if err != nil {
		return fmt.Errorf("error unmarshaling response: %v", err)
	}

	return nil
}

func getItemOption(items []PackageItem, category string) string {
	for _, item := range items {
		if item.Category == category {
			return item.Option
		}
	}
	return "N/A"
}

func getPublicIP(subnets []IPSubnet) string {
	for _, subnet := range subnets {
		if subnet.NetworkType == "public" {
			return subnet.Block
		}
	}
	return "N/A"
}

func rebootServer(serverID string) (int, RebootResponse, error) {
    client := resty.New()
    resp, err := client.R().
        SetBasicAuth(credentials.Username, credentials.Password).
        Get(fmt.Sprintf("%s/%s/restart", apiUrl, serverID))
    if err != nil {
        return 0, RebootResponse{}, fmt.Errorf("error making request: %v", err)
    }

    statusCode := resp.StatusCode()

    var rebootResp RebootResponse
    err = json.Unmarshal(resp.Body(), &rebootResp)
    if err != nil {
        return statusCode, RebootResponse{}, fmt.Errorf("error unmarshaling response: %v", err)
    }

    return statusCode, rebootResp, nil
}

func powerOffServer(serverID string) {
	// Implement power off API call here
	showMessage(fmt.Sprintf("Powering off server %s", serverID))
}

func powerOnServer(serverID string) {
	// Implement power on API call here
	showMessage(fmt.Sprintf("Powering on server %s", serverID))
}

func showMessage(message string) {
    modal := tview.NewModal().
        SetText(message).
        AddButtons([]string{"OK"}).
        SetDoneFunc(func(buttonIndex int, buttonLabel string) {
            pages.SwitchToPage("serverList")
        })

    pages.AddPage("message", modal, true, true)
    pages.SwitchToPage("message")
}

func showConfirmationDialog(server Server, onConfirm func()) {
    modal := tview.NewModal().
        SetText(fmt.Sprintf("Are you sure you want to restart server %s?", server.ServerID)).
        AddButtons([]string{"Yes", "No"}).
        SetDoneFunc(func(buttonIndex int, buttonLabel string) {
            if buttonLabel == "Yes" {
                onConfirm()
            } else {
                pages.SwitchToPage("serverMenu")
            }
        })

    pages.AddPage("confirmationDialog", modal, true, true)
    pages.SwitchToPage("confirmationDialog")
}
