package chess_server

import (
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type ChessServerBase interface {
	WSGameHandler(*websocket.Conn) /* WS Game Loop */
}

type ChessServer struct {
	Games       ChessGames
	MatchMaking MatchMaking
}

func (s *ChessServer) Init() {
	s.Games.Init()
	s.MatchMaking.Init(&s.Games)
}

type ChessServerContext struct {
	echo.Context
	Server *ChessServer
}

type WSJoinGameData struct {
	T        string `json:"type"`
	GameId   uint64 `json:"game_id"`
	PlayerId uint64 `json:"player_id"`
}

func (s *ChessServer) WSGameHandler(c echo.Context) error {
	/* Read the Join game message from client */
	cc := c.(*ChessServerContext)

	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	var joinMsg WSJoinGameData
	err = ws.ReadJSON(&joinMsg)
	game := cc.Server.Games.Games[joinMsg.GameId]
	if err != nil {
		cc.Logger().Error(`{"reason": "Failed to receive join message"}`)
	}
	if joinMsg.PlayerId != game.WhitePlayerId && joinMsg.PlayerId != game.BlackPlayerId {
		cc.Logger().Error(`{"reason": "Invalid player_id"}`)
	} else {
		cc.Logger().Info(fmt.Sprintf("Player %d joined", joinMsg.PlayerId))
	}

	stream, err := game.GetPlayerStream(joinMsg.PlayerId)
	if err != nil {
		cc.Logger().Error("Failed to connect to game state")
	}

	/* If player is white, read a move message from WS. Apply the move. Read update from event stream to confirm move has been made */
	if joinMsg.PlayerId == game.WhitePlayerId {
		var moveMsg WSMoveUpdate
		err = ws.ReadJSON(&moveMsg)
		if err != nil {
			cc.Logger().Error(`{"reason": "Failed to receive move from client"}`)
		} else {
			cc.Logger().Info(fmt.Sprintf("Player %d entered move %s", joinMsg.PlayerId, moveMsg.Move))
		}

		if !game.MakeMove(moveMsg) {
			cc.Logger().Error(fmt.Sprintf("Invalid move %s", moveMsg.Move))
		}

		event := <-stream
		if event.Type() != "move_update" {
			cc.Logger().Error("Unexpected update message")
		}
		ws.WriteJSON(&event)
	}

	/* Loop
	- Read update from stream. It should either be a move update, or a result update if the game ended
	- Forward the update message to the WS client
	- Read a move message from WS. Apply the move. Read update from event stream to confirm move has been made
	*/
	for {
		event := <-stream
		ws.WriteJSON(&event)
		if event.Type() == "game_result_update" {
			break
		}

		var moveMsg WSMoveUpdate
		err = ws.ReadJSON(&moveMsg)
		if err != nil {
			cc.Logger().Error(`{"reason": "Failed to receive move from client"}`)
		} else {
			cc.Logger().Info(fmt.Sprintf("Player %d entered move %s", joinMsg.PlayerId, moveMsg.Move))
		}

		if !game.MakeMove(moveMsg) {
			cc.Logger().Error(fmt.Sprintf("Invalid move %s", moveMsg.Move))
		}

		event = <-stream
		if event.Type() != "move_update" {
			cc.Logger().Error("Unexpected update message")
		}
		ws.WriteJSON(&event)
	}

	return nil
}
