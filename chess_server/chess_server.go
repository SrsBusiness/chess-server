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
	ChessGamesController  ChessGamesController
	MatchMakingController MatchMakingController
}

func (s *ChessServer) Init() {
	s.ChessGamesController.Init()
	s.MatchMakingController.Init(&s.ChessGamesController.Events)
}

type ChessServerContext struct {
	echo.Context
	Server *ChessServer
}

func (s *ChessServer) PlayerLoop(
	gameControllerChannel *ChessGamesControllerChannel,
	wsIn <-chan struct {
		GameUpdate
		string
	},
	wsOut chan<- GameUpdate,
	logger echo.Logger) {

	clientUpdate, ok := <-wsIn
	if !ok {
		logger.Error("Failed to receive join update")
		return
	} else if clientUpdate.string != "player_joined_update" {
		logger.Error(fmt.Sprintf("Expected join update, instead received %s", clientUpdate.GameUpdate))
		return
	}
	joinMsg := clientUpdate.GameUpdate.(GamePlayerJoinedUpdate)

    /* TODO: Use gameId to get the event stream for the game */
	gameId := joinMsg.GameId
	playerId := joinMsg.PlayerId

	eventsIn, eventsOut, err := gameControllerChannel.PlayerJoin(joinMsg)
	if err != nil {
		logger.Error(fmt.Sprintf("Could not join game %d as player %d: %s", gameId, playerId, err))
		return
	} else {
		logger.Info(fmt.Sprintf("Player %d joined game %d", playerId, gameId))
	}
	defer eventsIn.PlayerLeave(GamePlayerLeftUpdate{GameId: gameId, PlayerId: playerId})

	for {
		select {
		case update := <-eventsOut:
			wsOut <- update.GameUpdate
            logger.Info(update.string)
			if update.string == "result_update" {
				return
			}
		case clientUpdate, ok := <-wsIn:
			if !ok {
				logger.Error(`{"reason": "Failed to receive move from client"}`)
				return
			} else if clientUpdate.string == "EOF" {
				return
			} else if clientUpdate.string != "move_update" { /* TODO: support updates like resign, draw offer, etc. */
				logger.Error(fmt.Sprintf("Expected move update, instead received %s", clientUpdate.GameUpdate))
				return
			}
			moveUpdate := clientUpdate.GameUpdate.(GameMoveUpdate)
			logger.Info(fmt.Sprintf("Player %d entered move %s", playerId, moveUpdate.Move))

			if err := eventsIn.MakeMove(moveUpdate); err != nil {
				logger.Error(fmt.Sprintf("Invalid move %s", err))
				return
			}
		}
	}
}

func (s *ChessServer) SpectateLoop(
    gameControllerChannel *ChessGamesControllerChannel,
	wsIn <-chan struct {
		GameUpdate
		string
	},
	wsOut chan<- GameUpdate,
	logger echo.Logger) {

	clientUpdate, ok := <-wsIn
	if !ok {
		logger.Error("Failed to receive spectator join update")
		return
	} else if clientUpdate.string != "spectator_join_update" {
		logger.Error(fmt.Sprintf("Expected Spectator join update, instead received %s", clientUpdate.GameUpdate))
		return
	}
	joinMsg := clientUpdate.GameUpdate.(GameSpectatorJoinUpdate)

	spectator_id, eventsIn, eventsOut, err := gameControllerChannel.SpectatorJoin(joinMsg)
	if err != nil {
		logger.Error("Could not spectate game: %s", err)
		return
	}
	logger.Info(fmt.Sprintf("Spectator %d is now spectating game %d", spectator_id, joinMsg.GameId))
	defer eventsIn.SpectatorLeave(GameSpectatorLeftUpdate{GameId: joinMsg.GameId, SpectatorId: spectator_id})
	for {
		select {
		case update := <-eventsOut:
			wsOut <- update.GameUpdate
			if update.string == "result_update" {
				return
			}
		case clientMsg := <-wsIn:
			/* Unless it is EOF ignore */
			if clientMsg.string == "EOF" {
				return
			}
		}
	}
}

func (s *ChessServer) WSHandler(f func(
	gameController *ChessGamesControllerChannel,
	wsIn <-chan struct {
		GameUpdate
		string
	},
	wsOut chan<- GameUpdate,
	logger echo.Logger)) func(c echo.Context) error {
	return func(c echo.Context) error {
		ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return err
		}
		defer ws.Close()

		wsIn := make(chan struct {
			GameUpdate
			string
		})
		wsOut := make(chan GameUpdate)
		defer close(wsOut)
		cc := c.(*ChessServerContext)
		wsController := WSController{
			Ws:     ws,
			In:     wsIn,
			Out:    wsOut,
			Logger: cc.Logger(),
		}

		gameControllerChannel := &cc.Server.ChessGamesController.Events

		writerSignal := make(chan struct{})

		go wsController.WSReader()
		go wsController.WSWriter(writerSignal)

		/* this function will signal to the reader that the session is finished */
		f(gameControllerChannel, wsIn, wsOut, cc.Logger())
		/* signal to writer to finish */
		writerSignal <- struct{}{}
		return nil
	}
}
