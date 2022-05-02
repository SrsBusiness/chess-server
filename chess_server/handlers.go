package chess_server

import (
	"net/http"

	"strings"

	"github.com/labstack/echo/v4"
	"github.com/notnil/chess"
	"golang.org/x/net/websocket"
)

// Handler
func Hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

type GameIdResponse struct {
	T      string `json: "type" xml: "type"`
	GameId uint64 `json: "game_id" xml: "game_id"`
}

// Make asynchronous?
func FindMatch(c echo.Context) error {
	cc := c.(*ChessServerContext)
	response := make(chan uint64)
	cc.Server.MatchMaking.FindMatch(&response)

	gameId := <-response

	responseJSON := &GameIdResponse{
		T:      "game_id_response",
		GameId: gameId,
	}
	return cc.JSON(http.StatusOK, responseJSON)
}

func GameWIP(c echo.Context) error {
	//server := websocket.Server()

	//return c.String(http.StatusOK, "Finding Match")
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		game := chess.NewGame()
		var err error
		for game.Outcome() == chess.NoOutcome {
			// Player is white
			msg := ""
			err = websocket.Message.Receive(ws, &msg)
			if err != nil {
				c.Logger().Error(err)
				continue
			}

			if err = game.MoveStr(strings.TrimSpace(msg)); err != nil {
				c.Logger().Error(err)
				continue
			}

			if game.Outcome() != chess.NoOutcome {
				break
			}
			moves := game.ValidMoves()

			move := moves[0]
			game.Move(move)

			err = websocket.Message.Send(ws, move.String())
			if err != nil {
				c.Logger().Error(err)
			}

			err = websocket.Message.Send(ws, game.Position().Board().Draw())
			if err != nil {
				c.Logger().Error(err)
			}
		}
		err = websocket.Message.Send(ws, game.Position().Board().Draw())
		if err != nil {
			c.Logger().Error(err)
		}
		end_msg := ""
		if game.Outcome() == chess.WhiteWon {
			end_msg = "You win!"
		} else if game.Outcome() == chess.Draw {
			end_msg = "Draw!"
		} else {
			end_msg = "You lose!"
		}
		err = websocket.Message.Send(ws, end_msg)
		if err != nil {
			c.Logger().Error(err)
		}
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}
