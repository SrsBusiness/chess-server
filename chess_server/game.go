package chess_server

import (
	"errors"
	"strings"

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

type ChessGamesControllerRequest struct {
	Update   GameUpdate
	Response chan<- interface{}
}

type ChessGamesController struct {
	Games                map[uint64]*ChessGame
	Requests             chan ChessGamesControllerRequest
	NextAvailGameId      uint64
	NextAvailPlayerId    uint64
	NextAvailSpectatorId uint64
}

type ChessGame struct {
	GameId            uint64
	GameState         *chess.Game
	WhitePlayerId     uint64
	BlackPlayerId     uint64
	WhitePlayerStream chan struct {
		GameUpdate
		string
	}
	BlackPlayerStream chan struct {
		GameUpdate
		string
	}
	WhitePlayerJoined bool
	BlackPlayerJoined bool
	SpectatorStreams  map[uint64]chan struct {
		GameUpdate
		string
	}
}

func (g *ChessGamesController) Init() {
	g.Games = make(map[uint64]*ChessGame)
	g.Requests = make(chan ChessGamesControllerRequest)
}

type SpectatorJoinResponse struct {
	SpectatorId uint64
	Error       error
}

func (g *ChessGamesController) Run() {
	for {
		request := <-g.Requests
		update := request.Update
		response := request.Response
		switch update.(type) {
		case GameNewUpdate:
			response <- g.addNewGame()
		case GamePlayerJoinedUpdate:
			playerJoinedUpdate := update.(GamePlayerJoinedUpdate)
			response <- g.playerJoin(playerJoinedUpdate.GameId, playerJoinedUpdate.PlayerId)
		case GamePlayerLeftUpdate:
			playerLeftUpdate := update.(GamePlayerLeftUpdate)
			g.playerLeave(playerLeftUpdate.GameId, playerLeftUpdate.PlayerId)
		case GameSpectatorJoinUpdate:
			spectatorJoinUpdate := update.(GameSpectatorJoinUpdate)
			spectatorId, err := g.spectatorJoin(spectatorJoinUpdate.GameId)
			response <- SpectatorJoinResponse{SpectatorId: spectatorId, Error: err}
		case GameSpectatorLeftUpdate:
			spectatorLeftUpdate := update.(GameSpectatorLeftUpdate)
			g.spectatorLeave(spectatorLeftUpdate.GameId, spectatorLeftUpdate.SpectatorId)
		case GameMoveUpdate:
			moveUpdate := update.(GameMoveUpdate)
			response <- g.makeMove(moveUpdate)
		default:
			continue
		}
	}
}

func (g *ChessGamesController) AddNewGame() *ChessGame {
	response := make(chan interface{})
	g.Requests <- ChessGamesControllerRequest{Update: GameNewUpdate{}, Response: response}
	game := <-response
	close(response)
	return game.(*ChessGame)
}

func (g *ChessGamesController) PlayerJoin(update GamePlayerJoinedUpdate) error {
	response := make(chan interface{})
	g.Requests <- ChessGamesControllerRequest{Update: update, Response: response}
	err := <-response
	close(response)
	if err != nil {
		return err.(error)
	} else {
		return nil
	}
}

func (g *ChessGamesController) PlayerLeave(update GamePlayerLeftUpdate) {
	response := make(chan interface{})
	g.Requests <- ChessGamesControllerRequest{Update: update, Response: response}
	close(response)
}

func (g *ChessGamesController) SpectatorJoin(update GameSpectatorJoinUpdate) (uint64, error) {
	response := make(chan interface{})
	g.Requests <- ChessGamesControllerRequest{Update: update, Response: response}
	spectatorId := <-response
	close(response)
	return spectatorId.(SpectatorJoinResponse).SpectatorId, spectatorId.(SpectatorJoinResponse).Error
}

func (g *ChessGamesController) SpectatorLeave(update GameSpectatorLeftUpdate) {
	response := make(chan interface{})
	g.Requests <- ChessGamesControllerRequest{Update: update, Response: response}
	close(response)
}

func (g *ChessGamesController) MakeMove(update GameMoveUpdate) error {
	response := make(chan interface{})
	g.Requests <- ChessGamesControllerRequest{Update: update, Response: response}
	err := <-response
	close(response)
	if err != nil {
		return err.(error)
	} else {
		return nil
	}
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

/* TODO: think about rejoining after disconnect? */
func (g *ChessGamesController) playerJoin(gameId uint64, playerId uint64) error {
	game := g.Games[gameId]
	if playerId != game.WhitePlayerId && playerId != game.BlackPlayerId {
		return errors.New("Invalid Player Id")
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
		game.WhitePlayerJoined = true
		game.WhitePlayerStream = make(chan struct {
			GameUpdate
			string
		}, 128)
		game.WhitePlayerStream <- struct {
			GameUpdate
			string
		}{snapshot, "snapshot_update"}
	}
	if playerId == game.BlackPlayerId {
		game.BlackPlayerJoined = true
		game.BlackPlayerStream = make(chan struct {
			GameUpdate
			string
		}, 128)
		game.BlackPlayerStream <- struct {
			GameUpdate
			string
		}{snapshot, "snapshot_update"}
	}
	return nil
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
	game.BroadcastUpdate(playerLeftUpdate, "player_left_update")
	if playerId == game.WhitePlayerId {
		close(game.WhitePlayerStream)
		game.WhitePlayerStream = nil
	} else {
		close(game.BlackPlayerStream)
		game.BlackPlayerStream = nil
	}
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

func (g *ChessGamesController) spectatorJoin(gameId uint64) (uint64, error) {
	spectatorId := g.NextAvailSpectatorId
	g.NextAvailSpectatorId += 1
	game, ok := g.Games[gameId]
	if !ok {
		return 0, errors.New("Invalid GameId")
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
	return spectatorId, nil
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
}

func (g *ChessGamesController) Turn(gameId uint64) uint64 {
	game := g.Games[gameId]
	if game.GameState.Position().Turn() == chess.White {
		return game.WhitePlayerId
	} else {
		return game.BlackPlayerId
	}
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

func (g *ChessGamesController) makeMove(move GameMoveUpdate) error {
	game := g.Games[move.GameId]
	if move.PlayerId == game.WhitePlayerId {
		if move.PlayerColor != "w" {
			return errors.New("Invalid Player color")
		}
	} else if move.PlayerId == game.BlackPlayerId {
		if move.PlayerColor != "b" {
			return errors.New("Invalid Player color")
		}
	} else {
		return errors.New("Invalid Player Id")
	}

	color := strings.ToLower(game.GameState.Position().Turn().String())
	if color != move.PlayerColor {
		return errors.New("Player color does not match what's on the server")
	}

	err := game.GameState.MoveStr(move.Move)
	if err != nil {
		return errors.New("Invalid move")
	}
	move.FEN = game.GameState.FEN()

	game.BroadcastUpdate(move, "move_update")

	/* Check if game has ended - if so send a follow-up update */
	if game.GameState.Outcome() != chess.NoOutcome {
		resultUpdate := GameResultUpdate{
			Result: game.GameState.Outcome().String(),
			FEN:    game.GameState.Position().String(),
		}
		game.BroadcastUpdate(resultUpdate, "result_update")
		delete(g.Games, move.GameId)
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
		GameState:         chess.NewGame(),
		GameId:            gameId,
		WhitePlayerId:     g.NextAvailPlayerId,
		BlackPlayerId:     g.NextAvailPlayerId + 1,
		WhitePlayerStream: nil,
		BlackPlayerStream: nil,
		WhitePlayerJoined: false,
		BlackPlayerJoined: false,
		SpectatorStreams: make(map[uint64]chan struct {
			GameUpdate
			string
		}),
	}
	g.Games[gameId] = &newGame
	g.NextAvailPlayerId += 2
	return &newGame
}
