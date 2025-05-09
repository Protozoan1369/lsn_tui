package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/go-resty/resty/v2"
	"github.com/rivo/tview"
)

type RebootResponse struct {
	Status bool  `json:"status"`
	Message string `json:"message"`
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

	// Show loading screen
	showLoadingScreen("Fetching servers... Please wait")
	
	// Start the app with loading screen
	go func() {
		// Fetch servers in background
		if err := fetchServers(); err != nil {
			app.Stop()
			log.Fatalf("Error fetching servers: %v", err)
		}
		
		// Update UI on main thread
		app.QueueUpdateDraw(func() {
			showServerList()
		})
	}()

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
		serverTable.SetCell(i+1, 0, tview.NewTableCell(server.ServerID))
		serverTable.SetCell(i+1, 1, tview.NewTableCell(server.Package.Name))
		serverTable.SetCell(i+1, 2, tview.NewTableCell(server.ManagementIP))
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

func showLoadingScreen(message string) {
    // Create a flex layout for the loading screen
    flex := tview.NewFlex().
        SetDirection(tview.FlexRow)
    
    // Add empty space at the top (for vertical centering)
    flex.AddItem(nil, 0, 1, false)
    
    // Create a text view with a loading message
    loadingText := tview.NewTextView().
        SetText(message).
        SetTextAlign(tview.AlignCenter).
        SetTextColor(tcell.ColorWhite)
    
    // Create a text view for the timer
    timerText := tview.NewTextView().
        SetTextAlign(tview.AlignCenter).
        SetTextColor(tcell.ColorYellow)
    
    // Create a horizontal flex to center the message text
    horizontalFlex := tview.NewFlex().
        SetDirection(tview.FlexColumn)
    
    // Add empty space on the left (for horizontal centering)
    horizontalFlex.AddItem(nil, 0, 1, false)
    // Add the loading text
    horizontalFlex.AddItem(loadingText, 30, 1, false)
    // Add empty space on the right (for horizontal centering)
    horizontalFlex.AddItem(nil, 0, 1, false)
    
    // Create a horizontal flex to center the timer text
    timerFlex := tview.NewFlex().
        SetDirection(tview.FlexColumn)
    
    // Add empty space on the left (for horizontal centering)
    timerFlex.AddItem(nil, 0, 1, false)
    // Add the timer text
    timerFlex.AddItem(timerText, 30, 1, false)
    // Add empty space on the right (for horizontal centering)
    timerFlex.AddItem(nil, 0, 1, false)
    
    // Add the horizontal flexes to the main vertical flex
    flex.AddItem(horizontalFlex, 3, 1, false)
    flex.AddItem(timerFlex, 1, 1, false)
    // Add empty space at the bottom (for vertical centering)
    flex.AddItem(nil, 0, 1, false)
    
    // Add the flex layout to the pages
    pages.AddPage("loading", flex, true, true)
    pages.SwitchToPage("loading")
    
    // Start the timer in a separate goroutine
    go func() {
        startTime := time.Now()
        ticker := time.NewTicker(100 * time.Millisecond)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                elapsed := time.Since(startTime)
                
                // Format the elapsed time
                var timeStr string
                if elapsed.Minutes() < 1 {
                    // Less than a minute - show seconds
                    timeStr = fmt.Sprintf("Elapsed: %.1f seconds", elapsed.Seconds())
                } else {
                    // One minute or more - show minutes and seconds
                    minutes := int(elapsed.Minutes())
                    seconds := int(elapsed.Seconds()) % 60
                    timeStr = fmt.Sprintf("Elapsed: %d min %d sec", minutes, seconds)
                }
                
                // Update the timer text
                app.QueueUpdateDraw(func() {
                    timerText.SetText(timeStr)
                })
                
                // Check if we're no longer on the loading page
                currentPage, _ := pages.GetFrontPage()
                if currentPage != "loading" {
                    return
                }
            }
        }
    }()
}
