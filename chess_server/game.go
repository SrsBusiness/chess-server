package chess_server

import (
	"errors"
	"strings"
    "fmt"
	"github.com/notnil/chess"
)

// WS messages

type GameUpdate interface {
}

type GameNewUpdate struct {
}

type GameSyncUpdate struct {
	GameId        uint64 `json:"game_id"`
	WhitePlayerId uint64 `json:"white_player_id"`
	BlackPlayerId uint64 `json:"black_player_id"`
	FEN           string `json:"fen"`
}

type GameMoveUpdate struct {
	GameId      uint64 `json:"game_id"`
	Move        string `json:"move"`
	PlayerId    uint64 `json:"player_id"`
	PlayerColor string `json:"player_color"`
	FEN         string `json:"fen"`
}

func (u GameMoveUpdate) Type() string {
	return "move_update"
}

type GameResultUpdate struct {
	Result string `json:"result"`
	FEN    string `json:"fen"`
}

func (u GameResultUpdate) Type() string {
	return "game_result_update"
}

type EventChannel struct {
    C chan ChessGamesControllerRequest
}

type ChessGamesControllerChannel struct {
    EventChannel
}

func (c *ChessGamesControllerChannel) PlayerJoin(update GamePlayerJoinedUpdate) (*ChessGameChannel, chan struct {
	GameUpdate
	string
}, error) {
	response := make(chan interface{})
	c.C <- ChessGamesControllerRequest{Update: update, Response: response}
	ret := <-response
	close(response)
	return ret.(PlayerJoinResponse).EventsIn, ret.(PlayerJoinResponse).EventsOut, ret.(PlayerJoinResponse).Error
}

func (c *ChessGamesControllerChannel) PlayerLeave(update GamePlayerLeftUpdate) {
	response := make(chan interface{})
	c.C <- ChessGamesControllerRequest{Update: update, Response: response}
	close(response)
}

func (c *ChessGamesControllerChannel) SpectatorJoin(update GameSpectatorJoinUpdate) (uint64, chan struct {
	GameUpdate
	string
}, error) {
	response := make(chan interface{})
	c.C <- ChessGamesControllerRequest{Update: update, Response: response}
	ret := <-response
	close(response)
	return ret.(SpectatorJoinResponse).SpectatorId, ret.(SpectatorJoinResponse).SpectatorStream, ret.(SpectatorJoinResponse).Error
}

func (c *ChessGamesControllerChannel) SpectatorLeave(update GameSpectatorLeftUpdate) {
	response := make(chan interface{})
	c.C <- ChessGamesControllerRequest{Update: update, Response: response}
	close(response)
}

type ChessGameChannel struct {
    EventChannel
}

func (c *ChessGameChannel) MakeMove(update GameMoveUpdate) error {
	response := make(chan interface{})
	c.C <- ChessGamesControllerRequest{Update: update, Response: response}
	err := <-response
	close(response)
	if err != nil {
		return err.(error)
	} else {
		return nil
	}
}

type ChessGamesControllerRequest struct {
	Update   GameUpdate
	Response chan<- interface{}
}

/* 
 * ChessGamesController is a long-lived agent that is responsible for managing
 * (creating and destroying) resources needed to run a chess game. It listens
 * for player/spectator join and leave updates
 */
type ChessGamesController struct {
	Games                map[uint64]*ChessGame
	Events               ChessGamesControllerChannel 
	NextAvailGameId      uint64
	NextAvailPlayerId    uint64
	NextAvailSpectatorId uint64
}

/*
 * Represents a live chess game. Manages updates to the game while it is still
 * live (move updates, draw offers, resignation, etc)
 */
type ChessGame struct {
	GameId            uint64
	GameState         *chess.Game
	WhitePlayerId     uint64
	BlackPlayerId     uint64

    Events            ChessGameChannel
    
    /* Used to signal to the controller thread to delete this game */
    ControllerRequests *ChessGamesControllerChannel

	WhitePlayerStream chan struct {
		GameUpdate
		string
	}
	BlackPlayerStream chan struct {
		GameUpdate
		string
	}
	WhitePlayerConnected bool
	BlackPlayerConnected bool
	SpectatorStreams     map[uint64]chan struct {
		GameUpdate
		string
	}
}

func (g *ChessGamesController) Init() {
	g.Games = make(map[uint64]*ChessGame)
	g.Events.C = make(chan ChessGamesControllerRequest)
}

type SpectatorJoinResponse struct {
	SpectatorId     uint64
	SpectatorStream chan struct {
		GameUpdate
		string
	}
	Error error
}

type PlayerJoinResponse struct {
    EventsIn *ChessGameChannel
	EventsOut chan struct {
		GameUpdate
		string
	}
	Error error
}

func (g *ChessGamesController) Run() {
	for {
		request := <-g.Events.C
		update := request.Update
		response := request.Response
		switch update.(type) {
		case GameNewUpdate:
			response <- g.addNewGame()
		case GamePlayerJoinedUpdate:
			playerJoinedUpdate := update.(GamePlayerJoinedUpdate)
			eventsIn, eventsOut, err := g.playerJoin(playerJoinedUpdate.GameId, playerJoinedUpdate.PlayerId)
            response <- PlayerJoinResponse{EventsIn: eventsIn, EventsOut: eventsOut, Error: err}
		case GamePlayerLeftUpdate:
			playerLeftUpdate := update.(GamePlayerLeftUpdate)
			g.playerLeave(playerLeftUpdate.GameId, playerLeftUpdate.PlayerId)
		case GameSpectatorJoinUpdate:
			spectatorJoinUpdate := update.(GameSpectatorJoinUpdate)
			spectatorId, spectatorStream, err := g.spectatorJoin(spectatorJoinUpdate.GameId)
			response <- SpectatorJoinResponse{SpectatorId: spectatorId, SpectatorStream: spectatorStream, Error: err}
		case GameSpectatorLeftUpdate:
			spectatorLeftUpdate := update.(GameSpectatorLeftUpdate)
			g.spectatorLeave(spectatorLeftUpdate.GameId, spectatorLeftUpdate.SpectatorId)	
        case GameDeleteRequest:
            deleteRequest := update.(GameDeleteRequest)
            g.deleteGame(deleteRequest.GameId)
		default:
            /* log error ? */
			continue
		}
	}
}

func (c *ChessGamesControllerChannel) AddNewGame() *ChessGame {
	response := make(chan interface{})
	c.C <- ChessGamesControllerRequest{Update: GameNewUpdate{}, Response: response}
	game := <-response
	close(response)
	return game.(*ChessGame)
}

func (c *ChessGamesControllerChannel) DeleteNewGame(gameId uint64) {
    c.C <- ChessGamesControllerRequest{Update: GameDeleteRequest{GameId: gameId}, Response: nil}
}

func (g *ChessGame) Run() {
    fmt.Println("Starting game")
    for !g.Finished() || g.WhitePlayerConnected && g.BlackPlayerConnected || len(g.SpectatorStreams) > 0 {
        request := <-g.Events.C
        update := request.Update
        response := request.Response
        switch update.(type) {
        case GameMoveUpdate:
		    moveUpdate := update.(GameMoveUpdate)
		    response <- g.makeMove(moveUpdate)
        default:
            /* log error ? */
            continue
        }
    }
    /* If game is finished and all players and spectators have left, clean up */
    fmt.Println("Deleting game")
    g.ControllerRequests.DeleteNewGame(g.GameId)    	
}

func (g *ChessGamesController) GetGameSyncUpdate(gameId uint64) GameSyncUpdate {
	game := g.Games[gameId]
	return GameSyncUpdate{
		GameId:        gameId,
		WhitePlayerId: game.WhitePlayerId,
		BlackPlayerId: game.BlackPlayerId,
		FEN:           g.GetFEN(gameId),
	}
}

func (g *ChessGamesController) GetFEN(gameId uint64) string {
	game := g.Games[gameId]
	return game.GameState.FEN()
}

func (g *ChessGamesController) GetPlayerStream(gameId uint64, playerId uint64) (chan struct {
	GameUpdate
	string
}, error) {
	game := g.Games[gameId]
	if playerId == game.WhitePlayerId {
		return game.WhitePlayerStream, nil
	} else if playerId == game.BlackPlayerId {
		return game.BlackPlayerStream, nil
	} else {
		return nil, errors.New("Invalid Player Id")
	}
}

func (g *ChessGamesController) GetSpectatorStream(gameId uint64, spectatorId uint64) (chan struct {
	GameUpdate
	string
}, error) {
	game := g.Games[gameId]
	stream, ok := game.SpectatorStreams[spectatorId]
	if !ok {
		return nil, errors.New("Invalid Spectator Id")
	}
	return stream, nil
}

type GamePlayerJoinedUpdate struct {
	GameId   uint64 `json:"game_id"`
	PlayerId uint64 `json:"player_id"`
}

type GamePlayerLeftUpdate struct {
	GameId   uint64 `json:"game_id"`
	PlayerId uint64 `json:"player_id"`
}

func (g *ChessGamesController) playerJoin(gameId uint64, playerId uint64) (*ChessGameChannel, chan struct {
	GameUpdate
	string
}, error) {
	game := g.Games[gameId]
	if playerId != game.WhitePlayerId && playerId != game.BlackPlayerId {
		return nil, nil, errors.New("Invalid Player Id")
	}
	playerJoinedUpdate := GamePlayerJoinedUpdate{
		GameId:   gameId,
		PlayerId: playerId,
	}
	game.BroadcastUpdate(playerJoinedUpdate, "player_joined_update")
	snapshot := GameSyncUpdate{
		GameId: gameId,
		FEN:    g.GetFEN(gameId),
	}
	if playerId == game.WhitePlayerId {
		game.WhitePlayerConnected = true
		game.WhitePlayerStream = make(chan struct {
			GameUpdate
			string
		}, 128)
		game.WhitePlayerStream <- struct {
			GameUpdate
			string
		}{snapshot, "snapshot_update"}
		return &game.Events, game.WhitePlayerStream, nil
	} else if playerId == game.BlackPlayerId {
		game.BlackPlayerConnected = true
		game.BlackPlayerStream = make(chan struct {
			GameUpdate
			string
		}, 128)
		game.BlackPlayerStream <- struct {
			GameUpdate
			string
		}{snapshot, "snapshot_update"}
		return &game.Events, game.BlackPlayerStream, nil
	} else {
		return nil, nil, errors.New("Unreachable")
	}
}

func (g *ChessGamesController) playerLeave(gameId uint64, playerId uint64) error {
	game := g.Games[gameId]
	if playerId != game.WhitePlayerId && playerId != game.BlackPlayerId {
		return errors.New("Invalid Player Id")
	}
	playerLeftUpdate := GamePlayerLeftUpdate{
		GameId:   gameId,
		PlayerId: playerId,
	}
	if playerId == game.WhitePlayerId {
		close(game.WhitePlayerStream)
		game.WhitePlayerStream = nil
		game.WhitePlayerConnected = false
	} else {
		close(game.BlackPlayerStream)
		game.BlackPlayerStream = nil
		game.BlackPlayerConnected = false
	}
	game.BroadcastUpdate(playerLeftUpdate, "player_left_update")	

	return nil
}

type GameSpectatorJoinedUpdate struct {
	GameId      uint64 `json:"game_id"`
	SpectatorId uint64 `json:"spectator_id"`
}

type GameSpectatorJoinUpdate struct {
	GameId uint64 `json:"game_id"`
}

type GameSpectatorLeftUpdate struct {
	GameId      uint64 `json:"game_id"`
	SpectatorId uint64 `json:"spectator_id"`
}

type GameDeleteRequest struct {
    GameId      uint64
}

func (g *ChessGamesController) spectatorJoin(gameId uint64) (uint64, chan struct {
	GameUpdate
	string
}, error) {
	spectatorId := g.NextAvailSpectatorId
	g.NextAvailSpectatorId += 1
	game, ok := g.Games[gameId]
	if !ok {
		return 0, nil, errors.New("Invalid GameId")
	}
	spectatorStream := make(chan struct {
		GameUpdate
		string
	}, 128)
	game.SpectatorStreams[spectatorId] = spectatorStream
	spectatorJoinedUpdate := GameSpectatorJoinedUpdate{
		GameId:      gameId,
		SpectatorId: spectatorId,
	}
	game.BroadcastUpdate(spectatorJoinedUpdate, "spectator_joined_update")
	snapshot := GameSyncUpdate{
		GameId: gameId,
		FEN:    g.GetFEN(gameId),
	}
	spectatorStream <- struct {
		GameUpdate
		string
	}{snapshot, "snapshot_update"}
	return spectatorId, spectatorStream, nil
}

func (g *ChessGamesController) spectatorLeave(gameId uint64, spectatorId uint64) {
	game := g.Games[gameId]
	close(game.SpectatorStreams[spectatorId])
	delete(game.SpectatorStreams, spectatorId)
	spectatorLeftUpdate := GameSpectatorLeftUpdate{
		GameId:      gameId,
		SpectatorId: spectatorId,
	}
	game.BroadcastUpdate(spectatorLeftUpdate, "spectator_left")
	/* If game is finished and all players and spectators have left, clean up */
	if game.Finished() && !game.WhitePlayerConnected && !game.BlackPlayerConnected && len(game.SpectatorStreams) == 0 {
		delete(g.Games, gameId)
	}
}

func (g *ChessGamesController) Turn(gameId uint64) uint64 {
	game := g.Games[gameId]
	if game.GameState.Position().Turn() == chess.White {
		return game.WhitePlayerId
	} else {
		return game.BlackPlayerId
	}
}

func (g *ChessGame) Finished() bool {
	return g.GameState.Outcome() != chess.NoOutcome
}

func (g *ChessGame) BroadcastUpdate(update GameUpdate, T string) {
	updateMsg := struct {
		GameUpdate
		string
	}{update, T}
	if g.WhitePlayerStream != nil {
		g.WhitePlayerStream <- updateMsg
	}
	if g.BlackPlayerStream != nil {
		g.BlackPlayerStream <- updateMsg
	}
	for _, c := range g.SpectatorStreams {
		if c != nil {
			c <- updateMsg
		}
	}
}

func (g *ChessGame) makeMove(move GameMoveUpdate) error {
    if g.GameId != move.GameId {
        return errors.New("Invalid Game id")
    }
	if move.PlayerId == g.WhitePlayerId {
		if move.PlayerColor != "w" {
			return errors.New("Invalid Player color")
		}
	} else if move.PlayerId == g.BlackPlayerId {
		if move.PlayerColor != "b" {
			return errors.New("Invalid Player color")
		}
	} else {
		return errors.New("Invalid Player Id")
	}

	color := strings.ToLower(g.GameState.Position().Turn().String())
	if color != move.PlayerColor {
		return errors.New("Player color does not match what's on the server")
	}

	err := g.GameState.MoveStr(move.Move)
	if err != nil {
		return errors.New("Invalid move")
	}
	move.FEN = g.GameState.FEN()

	g.BroadcastUpdate(move, "move_update")

	/* Check if game has ended - if so send a follow-up update */
	if g.Finished() {
		resultUpdate := GameResultUpdate{
			Result: g.GameState.Outcome().String(),
			FEN:    g.GameState.Position().String(),
		}
		g.BroadcastUpdate(resultUpdate, "result_update")
	}
	return nil
}

func (g *ChessGamesController) GetGame(gameId uint64) (*ChessGame, error) {
	game, ok := g.Games[gameId]
	if !ok {
		return nil, errors.New("Invalid Game Id")
	}
	return game, nil

}

func (g *ChessGamesController) addNewGame() *ChessGame {
	/* Because we just have one Matchmaking goroutine calling this function we don't need to worry about
	 * synchronization yet
	 */
	gameId := g.NextAvailGameId
	g.NextAvailGameId += 1
	newGame := ChessGame{
		GameState:            chess.NewGame(),
		GameId:               gameId,
		WhitePlayerId:        g.NextAvailPlayerId,
		BlackPlayerId:        g.NextAvailPlayerId + 1,
		WhitePlayerStream:    nil,
		BlackPlayerStream:    nil,
		WhitePlayerConnected: false,
		BlackPlayerConnected: false,
		SpectatorStreams: make(map[uint64]chan struct {
			GameUpdate
			string
		}),
        ControllerRequests: &g.Events,
	}
    newGame.Events.C = make(chan ChessGamesControllerRequest)
	g.Games[gameId] = &newGame
	g.NextAvailPlayerId += 2
    go newGame.Run()
	return &newGame
}

func (g *ChessGamesController) deleteGame(gameId uint64) {
    delete(g.Games, gameId)
}
