package chess_server

import (
	"encoding/json"
	"errors"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type WSMessage struct {
	T      string          `json:"type"`
	Update json.RawMessage `json:"update"`
}

type WSController struct {
	Ws *websocket.Conn
	In chan<- struct {
		GameUpdate
		string
	}
	Out    <-chan GameUpdate
	Logger echo.Logger
}

func (c *WSController) ReadUnmarshal() (GameUpdate, string, error) {
	var msg WSMessage
	if err := c.Ws.ReadJSON(&msg); err != nil {
		return nil, "", err
	}
	switch msg.T {
	case "move_update":
		var moveUpdate GameMoveUpdate
		if err := json.Unmarshal(msg.Update, &moveUpdate); err != nil {
			return nil, "", err
		}
		return moveUpdate, msg.T, nil
	case "player_joined_update":
		var playerJoinUpdate GamePlayerJoinedUpdate
		if err := json.Unmarshal(msg.Update, &playerJoinUpdate); err != nil {
			return nil, "", err
		}
		return playerJoinUpdate, msg.T, nil
	case "spectator_join_update":
		var spectatorJoinUpdate GameSpectatorJoinUpdate
		if err := json.Unmarshal(msg.Update, &spectatorJoinUpdate); err != nil {
			return nil, "", err
		}
		return spectatorJoinUpdate, msg.T, nil
	default:
		return nil, "", errors.New("Unrecognized websocket message")
	}
}

func (c *WSController) WriteMarshal(update interface{}) error {
	updateData, err := json.Marshal(update)
	if err != nil {
		return err
	}
	msg := WSMessage{
		Update: updateData,
	}

	switch update.(type) {
	case GameMoveUpdate:
		msg.T = "move_update"
	case GameSyncUpdate:
		msg.T = "snapshot_update"
	case GameResultUpdate:
		msg.T = "result_update"
	case GameSpectatorJoinedUpdate:
		msg.T = "spectator_joined_update"
	case GameSpectatorLeftUpdate:
		msg.T = "spectator_left_update"
	case GamePlayerJoinedUpdate:
		msg.T = "player_joined_update"
	case GamePlayerLeftUpdate:
		msg.T = "player_left_update"
	default:
		return errors.New("Unsupported game update type")
	}
	return c.Ws.WriteJSON(msg)
}

/*
 * Helper goroutines to abstract the WS as channels
 * This may be useful since channels are first class citizens
 */
func (c *WSController) WSReader() {
	for {
		var inMsg GameUpdate
		inMsg, t, err := c.ReadUnmarshal()
		if err != nil {
			c.In <- struct {
				GameUpdate
				string
			}{nil, "EOF"}
			close(c.In)
			return
		}
		c.In <- struct {
			GameUpdate
			string
		}{inMsg, t}
	}
}

func (c *WSController) WSWriter() {
	for {
		outMsg := <-c.Out
		err := c.WriteMarshal(outMsg)
		if err != nil {
			return
		}
	}
}
