package chess_server

import (
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// Make asynchronous?
func FindMatch(c echo.Context) error {
	cc := c.(*ChessServerContext)
	response := make(chan MatchFoundResponse)
	cc.Server.MatchMakingController.FindMatch(response)

	responseJSON := <-response

	return cc.JSON(http.StatusOK, responseJSON)
}

var (
	upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
)
