package api

import (
	"dwc-server-advanced/gpcm"
	"net/http"
)

type defaultGameErrorEntry struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
}

func HandleErrorsPage(w http.ResponseWriter, r *http.Request) {
	serveHTMLPage(w, r, "/errors", errorsHTML)
}

func HandlePublicErrors(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	writePanelJSON(w, http.StatusOK, map[string]any{
		"custom_errors": gpcm.ListWWFCMessages(),
		"default_errors": []defaultGameErrorEntry{
			{
				Code:        "20100",
				Title:       "Unable to connect",
				Description: "The game could not connect to the online service.",
				Action:      "Check that the server is online and that your DNS or payload points the game to this server.",
			},
			{
				Code:        "20110",
				Title:       "Original Nintendo WFC discontinued",
				Description: "The game reached the old Nintendo Wi-Fi Connection service message instead of a working replacement service.",
				Action:      "Check the patch, DNS, or hosts setup. The client is probably not being routed to this server.",
			},
			{
				Code:        "20917",
				Title:       "Login failed",
				Description: "The game could not complete the account login step.",
				Action:      "Reconnect once. If it keeps happening, check the server logs, certificates, and profile-link state.",
			},
			{
				Code:        "23400",
				Title:       "Online service unavailable",
				Description: "The game could not reach the old online service.",
				Action:      "Check the patch, DNS, or hosts setup. The client is probably still trying to use the discontinued service.",
			},
			{
				Code:        "23404",
				Title:       "Matchmaking connection failed",
				Description: "The game reached online mode but could not complete matchmaking.",
				Action:      "Reconnect and check the room or matchmaking logs if it repeats.",
			},
			{
				Code:        "51030-51032",
				Title:       "Access point not found",
				Description: "The Wii could not find the configured wireless access point.",
				Action:      "Check the Wii connection settings and make sure the wireless network is visible and in range.",
			},
			{
				Code:        "51330-51332",
				Title:       "Wi-Fi authentication failed",
				Description: "The Wii could not authenticate with the router or access point.",
				Action:      "Check the Wi-Fi password, security type, router wireless mode, and wireless interference.",
			},
			{
				Code:        "52030-52032",
				Title:       "Internet connection failed",
				Description: "The Wii could not establish an internet connection.",
				Action:      "Check the Wi-Fi password, security settings, IP address settings, and router connection.",
			},
			{
				Code:        "52130-52132",
				Title:       "No internet after connecting",
				Description: "The Wii got a network address but could not reach the internet.",
				Action:      "Check DNS settings, router internet access, and whether other devices can browse the web.",
			},
			{
				Code:        "52230-52232",
				Title:       "Cannot connect to server",
				Description: "The Wii connected to the network but could not reach the online server.",
				Action:      "Check DNS routing, server availability, firewall rules, and whether the connection is timing out.",
			},
			{
				Code:        "60000",
				Title:       "Profile cannot be used",
				Description: "The saved game profile could not be accepted by the service.",
				Action:      "Create or link a valid game profile, then reconnect.",
			},
			{
				Code:        "61070",
				Title:       "Communication error",
				Description: "The game lost communication with the online service.",
				Action:      "Try again later, then check your internet connection and the server status if it repeats.",
			},
			{
				Code:        "71010",
				Title:       "Connection unstable",
				Description: "The game could not keep a stable connection to another player or to the online service.",
				Action:      "Check the network connection, NAT/firewall settings, and whether the other players are still connected.",
			},
			{
				Code:        "86420",
				Title:       "Player connection failed",
				Description: "The game could not complete peer-to-peer data transfer with another player.",
				Action:      "Check NAT type, firewall rules, router settings, and whether both players can connect to others.",
			},
		},
	})
}
