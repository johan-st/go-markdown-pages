package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"

	"github.com/charmbracelet/log"
)

type app struct {
	errorLogger  *log.Logger
	pagesHandler *handler
	errFatalChan chan<- error
}

func main() {
	// flagDev := flag.Bool("dev", false, "Run in development mode. Compiles templates on every request.")
	// flag.Parse()

	logger := log.New(os.Stderr)
	handler := newHandler(logger.WithPrefix("handler"))
	handler.prepareRoutes()
	errFatalChan := make(chan error)

	// fatal error handler
	go errFatal(errFatalChan)

	log.Fatal(
		tui(
			&app{
				errorLogger:  logger,
				pagesHandler: handler,
				errFatalChan: errFatalChan,
			}))

}

func runServer(h *handler) error {
	l := h.errorLogger.WithPrefix("http-server")

	srv := http.Server{
		Addr:    ":8080",
		Handler: h,
	}

	l.Info("Starting server", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("ListenAndServe: %s", err)
	}
	return fmt.Errorf("unexpected server shutdown...")
}

// TUI
func tui(app *app) error {

	fmt.Println("___________________")
	fmt.Println("| markdown server |")
	fmt.Println("|_________________|")
	fmt.Println("(type 'h' for help)")
	fmt.Print("enter command:")

	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		switch s.Text() {
		case "h":
			tuiHelp()
		case "q":
			log.Info("exiting...")
			os.Exit(0)
		case "s":
			log.Info("starting server...")
			log.Warn("TODO: implement graceful shutdown, already running handler, etc.")
			go func() {
				app.errFatalChan <- runServer(app.pagesHandler)
			}()
		case "r":
			log.Info("restarting server...")
			log.Warn("TODO: implement")

		case "1":
			log.Print("setting log level to Fatal")
			app.errorLogger.SetLevel(log.FatalLevel)
		case "2":
			log.Print("setting log level to Error")
			app.errorLogger.SetLevel(log.ErrorLevel)
		case "3":
			log.Print("setting log level to Warn")
			app.errorLogger.SetLevel(log.WarnLevel)
		case "4":
			log.Print("setting log level to Info")
			app.errorLogger.SetLevel(log.InfoLevel)
		case "5":
			log.Print("setting log level to Debug")
			app.errorLogger.SetLevel(log.DebugLevel)

		default:
			fmt.Println("unknown command (type 'h' for help)")
		}
		fmt.Print("enter command:")
	}
	return nil
}

func tuiHelp() {
	fmt.Println("--commands--")
	fmt.Println("h: help")
	fmt.Println("s: start server")
	fmt.Println("r: restart server")
	fmt.Println("q: exit")
	fmt.Println()
	fmt.Println("--logging level--")
	fmt.Println("1: log level Fatal")
	fmt.Println("2: log level Error")
	fmt.Println("3: log level Warn")
	fmt.Println("4: log level Info (default)")
	fmt.Println("5: log level Debug")
}

// error handlers
func errFatal(errChan <-chan error) {
	log.Fatal("Fatal error", "err", <-errChan)
}
