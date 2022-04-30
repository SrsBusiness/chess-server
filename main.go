package main

import (
	chess_server "github.com/SrsBusiness/chess_server/chess_server"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", chess_server.Hello)
	//e.GET("/find_match", find_match)

}
