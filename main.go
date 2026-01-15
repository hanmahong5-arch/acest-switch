package main

import (
	"codeswitch/services"
	"database/sql"
	"embed"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
	_ "modernc.org/sqlite"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/icon.png assets/icon-dark.png
var trayIcons embed.FS

type AppService struct {
	App *application.App
}

func (a *AppService) SetApp(app *application.App) {
	a.App = app
}

func (a *AppService) OpenSecondWindow() {
	if a.App == nil {
		fmt.Println("[ERROR] app not initialized")
		return
	}
	name := fmt.Sprintf("logs-%d", time.Now().UnixNano())
	win := a.App.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Logs",
		Name:      name,
		Width:     1024,
		Height:    800,
		MinWidth:  600,
		MinHeight: 300,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			TitleBar:                application.MacTitleBarHidden,
			Backdrop:                application.MacBackdropTransparent,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/#/logs",
	})
	win.Center()
}

// main function serves as the application's entry point. It initializes the application, creates a window,
// and starts a goroutine that emits a time-based event every second. It subsequently runs the application and
// logs any error that might occur.
func main() {
	appservice := &AppService{}

	// 初始化配置恢复服务 (Phase 4)
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".code-switch")

	// Get database connection for recovery service
	dbPath := filepath.Join(configDir, "app.db")
	db, dbErr := sql.Open("sqlite", dbPath+"?cache=shared&mode=rwc&_busy_timeout=5000")
	if dbErr != nil {
		log.Printf("[Recovery] Failed to open database: %v", dbErr)
	}

	var configRecovery *services.ConfigRecovery
	if db != nil {
		configRecovery = services.NewConfigRecovery(db, configDir)

		// Check for abnormal shutdown
		if crashed, err := configRecovery.DetectAbnormalShutdown(); err != nil {
			log.Printf("[Recovery] Failed to detect abnormal shutdown: %v", err)
		} else if crashed {
			log.Printf("[Recovery] ⚠️  Abnormal shutdown detected, attempting recovery...")
			if err := configRecovery.RecoverFromCrash(); err != nil {
				log.Printf("[Recovery] ❌ Recovery failed: %v", err)
			} else {
				log.Printf("[Recovery] ✓ Configuration recovered successfully")
			}
		} else {
			log.Printf("[Recovery] Normal startup detected")
		}
	}

	suiService, errt := services.NewSuiStore()
	if errt != nil {
		// 处理错误，比如日志或退出
	}
	providerService := services.NewProviderService()
	providerRelay := services.NewProviderRelayService(providerService, ":18100")
	claudeSettings := services.NewClaudeSettingsService(providerRelay.Addr())
	codexSettings := services.NewCodexSettingsService(providerRelay.Addr())
	geminiCliSettings := services.NewGeminiCLISettingsService()
	logService := services.NewLogService()
	autoStartService := services.NewAutoStartService()
	appSettings := services.NewAppSettingsService(autoStartService)
	mcpService := services.NewMCPService()
	skillService := services.NewSkillService()
	importService := services.NewImportService(providerService, mcpService)
	dockService := dock.New()
	versionService := NewVersionService()

	// 初始化同步服务（多端同步功能）
	syncSettingsService := services.NewSyncSettingsService()
	services.InitSyncIntegration(syncSettingsService)
	if si := services.GetSyncIntegration(); si != nil {
		providerRelay.SetSyncIntegration(si)
		if si.IsEnabled() {
			log.Printf("[Sync] Multi-device sync enabled")
		}
	}

	// 从配置读取各开关状态
	if settings, err := appSettings.GetAppSettings(); err == nil {
		// Body 日志开关
		providerRelay.SetBodyLogEnabled(settings.EnableBodyLog)
		if settings.EnableBodyLog {
			log.Printf("[Ailurus PaaS] Body logging enabled from config")
		}

		// NEW-API 统一网关配置
		if settings.NewAPIEnabled && settings.NewAPIURL != "" && settings.NewAPIToken != "" {
			providerRelay.SetNewAPIConfig(settings.NewAPIURL, settings.NewAPIToken)
			providerRelay.SetNewAPIEnabled(true)
			log.Printf("[Ailurus PaaS] NEW-API gateway mode enabled: %s", settings.NewAPIURL)
		}
	}

	// 执行数据迁移（将 Google Gemini 从 Codex 迁移到 Gemini-CLI）
	providerRelay.RunMigrations()

	go func() {
		if err := providerRelay.Start(); err != nil {
			log.Printf("provider relay start error: %v", err)
		}
	}()

	//fmt.Println(clipboardService)
	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	app := application.New(application.Options{
		Name:        "Ailurus PaaS",
		Description: "AI Provider Gateway for TUI and GUI Applications",
		Services: []application.Service{
			application.NewService(appservice),
			application.NewService(suiService),
			application.NewService(providerService),
			application.NewService(providerRelay),
			application.NewService(claudeSettings),
			application.NewService(codexSettings),
			application.NewService(geminiCliSettings),
			application.NewService(logService),
			application.NewService(appSettings),
			application.NewService(mcpService),
			application.NewService(skillService),
			application.NewService(importService),
			application.NewService(dockService),
			application.NewService(versionService),
			application.NewService(syncSettingsService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	app.OnShutdown(func() {
		_ = providerRelay.Stop()
		_ = syncSettingsService.ServiceShutdown()

		// Remove crash marker on normal shutdown (Phase 4)
		if configRecovery != nil {
			if err := configRecovery.RemoveCrashMarker(); err != nil {
				log.Printf("[Recovery] Failed to remove crash marker: %v", err)
			} else {
				log.Printf("[Recovery] Normal shutdown completed")
			}
		}
	})

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Ailurus PaaS",
		Width:     1024,
		Height:    800,
		MinWidth:  600,
		MinHeight: 300,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
	})
	var mainWindowCentered bool
	focusMainWindow := func() {
		if runtime.GOOS == "windows" {
			mainWindow.SetAlwaysOnTop(true)
			mainWindow.Focus()
			go func() {
				time.Sleep(150 * time.Millisecond)
				mainWindow.SetAlwaysOnTop(false)
			}()
			return
		}
		mainWindow.Focus()
	}
	showMainWindow := func(withFocus bool) {
		if !mainWindowCentered {
			mainWindow.Center()
			mainWindowCentered = true
		}
		if mainWindow.IsMinimised() {
			mainWindow.UnMinimise()
		}
		mainWindow.Show()
		if withFocus {
			focusMainWindow()
		}
		handleDockVisibility(dockService, true)
	}
	showMainWindow(false)

	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		mainWindow.Hide()
		handleDockVisibility(dockService, false)
		e.Cancel()
	})

	app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(event *application.ApplicationEvent) {
		showMainWindow(true)
	})

	app.Event.OnApplicationEvent(events.Mac.ApplicationDidBecomeActive, func(event *application.ApplicationEvent) {
		if mainWindow.IsVisible() {
			mainWindow.Focus()
			return
		}
		showMainWindow(true)
	})

	systray := app.SystemTray.New()
	// systray.SetLabel("Ailurus PaaS")
	systray.SetTooltip("Ailurus PaaS")
	if lightIcon := loadTrayIcon("assets/icon.png"); len(lightIcon) > 0 {
		systray.SetIcon(lightIcon)
	}
	if darkIcon := loadTrayIcon("assets/icon-dark.png"); len(darkIcon) > 0 {
		systray.SetDarkModeIcon(darkIcon)
	}

	trayMenu := application.NewMenu()
	trayMenu.Add("显示主窗口").OnClick(func(ctx *application.Context) {
		showMainWindow(true)
	})
	trayMenu.Add("退出").OnClick(func(ctx *application.Context) {
		app.Quit()
	})
	systray.SetMenu(trayMenu)

	systray.OnClick(func() {
		if !mainWindow.IsVisible() {
			showMainWindow(true)
			return
		}
		if !mainWindow.IsFocused() {
			focusMainWindow()
		}
	})

	appservice.SetApp(app)

	// Create a goroutine that emits an event containing the current time every second.
	// The frontend can listen to this event and update the UI accordingly.
	go func() {
		// for {
		// 	now := time.Now().Format(time.RFC1123)
		// 	app.EmitEvent("time", now)
		// 	time.Sleep(time.Second)
		// }
	}()

	// Run the application. This blocks until the application has been exited.
	err := app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}

func loadTrayIcon(path string) []byte {
	data, err := trayIcons.ReadFile(path)
	if err != nil {
		log.Printf("failed to load tray icon %s: %v", path, err)
		return nil
	}
	return data
}

func handleDockVisibility(service *dock.DockService, show bool) {
	if runtime.GOOS != "darwin" || service == nil {
		return
	}
	if show {
		service.ShowAppIcon()
	} else {
		service.HideAppIcon()
	}
}
