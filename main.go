package main

import (
	chess_server "github.com/SrsBusiness/chess_server/chess_server"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

func main() {
	// Start Backend
	var server chess_server.ChessServer
	server.Init()
	go server.MatchMaking.Run()

	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cc := &chess_server.ChessServerContext{Context: c, Server: &server}
			return next(cc)
		}
	})
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/find_match", chess_server.FindMatch)
	e.GET("/chess_game", server.WSGameHandler)

	// Start server
	e.Logger.SetLevel(log.INFO)
	e.Logger.Fatal(e.Start(":1323"))
}
