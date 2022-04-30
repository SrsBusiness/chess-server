package chessserver

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler
func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

func find_match(c echo.Context) error {
	return c.String(http.StatusOK, "Finding Match")
}
