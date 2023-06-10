package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/innermond/dots/http"
	"github.com/innermond/dots/postgres"
	"github.com/joho/godotenv"
)

const addr = ":8080"
var ServerGitHash = "not set"

func main() {
  var printVersion bool
  flag.BoolVar(&printVersion, "version", false, "print server version")
  flag.Parse()

  if printVersion {
    fmt.Printf("server bersion: %s\n", ServerGitHash)
    os.Exit(0)
  }

	pid := os.Getpid()
	fmt.Printf("PID: %d\n", pid)

	fmt.Println("initiating...")

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal(err)
	}

	clientId := os.Getenv("DOTS_GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("DOTS_GOOGLE_CLIENT_SECRET")
	tokenSecret := os.Getenv("DOTS_TOKEN_SECRET")
	if clientId == "" || clientSecret == "" || tokenSecret == ""{
		log.Fatal("app credentials are missing")
	}

	dsn := os.Getenv("DOTS_DSN")
	db := postgres.NewDB(dsn)
	if err := db.Open(); err != nil {
		panic(fmt.Errorf("cannot open database: %w", err))
	}

	server := http.NewServer()

	err = server.OpenSecureCookie()
	if err != nil {
		log.Fatal(err)
	}

	server.ClientID = clientId
	server.ClientSecret = clientSecret

	authService := postgres.NewAuthService(db)
	userService := postgres.NewUserService(db)
  tokenService := postgres.NewTokenService(db, tokenSecret)

	entryTypeService := postgres.NewEntryTypeService(db)
	entryService := postgres.NewEntryService(db)
	drainService := postgres.NewDrainService(db)
	companyService := postgres.NewCompanyService(db)
	deedService := postgres.NewDeedService(db)

	server.UserService = userService
	server.AuthService = authService
  server.TokenService = tokenService
	server.EntryTypeService = entryTypeService
	server.EntryService = entryService
	server.DrainService = drainService
	server.CompanyService = companyService
	server.DeedService = deedService

	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		cancel()
	}()

	go func() {
		fmt.Println("starting server...")
		log.Fatal(server.ListenAndServe(addr))
	}()

	<-ctx.Done()

	log.Println("closing server...")

	if err := server.Close(); err != nil {
		log.Printf("shutdown: %v\n", err)
	}

	if err := db.Close(); err != nil {
		log.Printf("shutdown: %v\n", err)
	}
}
