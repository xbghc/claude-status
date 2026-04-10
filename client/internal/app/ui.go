package app

import (
	"claude-status/internal/config"
	"claude-status/internal/monitor"
)

// UI defines the interface between application logic and the user interface.
// The app package drives business logic and calls UI methods to update display.
// UI implementations handle platform-specific rendering.
type UI interface {
	// Run starts the UI event loop. onReady is called when the UI is initialized.
	Run(onReady func(), onQuit func())

	// SetServers sets the list of available servers in the connection menu.
	SetServers(servers []config.ServerConfig)

	// ShowServerSelection displays the server selection prompt.
	ShowServerSelection()

	// SetConnecting updates the UI to show a connecting state.
	SetConnecting(msg string)

	// SetConnected updates the UI to show a connected state.
	SetConnected(msg string)

	// SetDisconnected updates the UI to show a disconnected state.
	SetDisconnected()

	// SetError updates the UI to show an error state.
	SetError(errType string, msg string)

	// SetIcon sets the tray icon. Valid values: "disconnected", "input-needed", "running".
	SetIcon(icon string)

	// SetStatusText sets the status text in the context menu.
	SetStatusText(text string)

	// SetTooltip sets the tray icon tooltip text.
	SetTooltip(text string)

	// UpdatePopup updates the popup window with session statuses.
	UpdatePopup(statuses []monitor.ProjectStatus)

	// QuitChan returns a channel that is closed when the user requests quit.
	QuitChan() <-chan struct{}

	// DisconnectChan returns a channel that receives when the user requests disconnect.
	DisconnectChan() <-chan struct{}

	// ServerSelectChan returns a channel that receives the selected server config.
	ServerSelectChan() <-chan config.ServerConfig
}
