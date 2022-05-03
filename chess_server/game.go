package chess_server

import (
	"errors"
	"fmt"
	"strings"

	"github.com/notnil/chess"
)

// WS messages
type WSGameEvent interface {
	Type() string
}

type WSGameStartUpdate struct {
	T      string `json:"type" xml:"type"`
	GameId uint64 `json:"game_id" xml:"game_id"`
}

func (u WSGameStartUpdate) Type() string {
	return "game_startup_update"
}

type WSMoveUpdate struct {
	T           string `json:"type" xml:"type"`
	GameId      uint64 `json:"game_id" xml:"game_id"`
	Move        string `json:"move" xml:"move"`
	PlayerId    uint64 `json:"player_id" xml:"player_id"`
	PlayerColor string `json:"player_color" xml:"player_color"`
	FEN         string `json:"fen" xml:"fen"`
}

func (u WSMoveUpdate) Type() string {
	return "move_update"
}

type WSGameResultUpdate struct {
	T      string `json:"type" xml:"type"`
	Result string `json:"result" xml:"result"`
	FEN    string `json:"fen" xml:"fen"`
}

func (u WSGameResultUpdate) Type() string {
	return "game_result_update"
}

type ChessGames struct {
	Games             map[uint64]ChessGame
	NextAvailGameId   uint64
	NextAvailPlayerId uint64
}

type ChessGame struct {
	GameId            uint64
	GameState         *chess.Game
	WhitePlayerId     uint64
	BlackPlayerId     uint64
	WhitePlayerStream chan WSGameEvent
	BlackPlayerStream chan WSGameEvent
	SpectatorStreams  []chan WSGameEvent
}

func (g *ChessGames) Init() {
	g.Games = make(map[uint64]ChessGame)
}

func (g *ChessGame) GetPlayerStream(playerId uint64) (chan WSGameEvent, error) {
	if playerId == g.WhitePlayerId {
		return g.WhitePlayerStream, nil
	} else if playerId == g.BlackPlayerId {
		return g.BlackPlayerStream, nil
	} else {
		return nil, errors.New("Invalid Player ID")
	}
}

func (g *ChessGame) BroadcastUpdate(update WSGameEvent) {
	g.WhitePlayerStream <- update
	g.BlackPlayerStream <- update
	fmt.Println("Broadcasting to all spectators")
	for _, c := range g.SpectatorStreams {
		c <- update
	}
	fmt.Println("Finished broadcasting")
}

func (g *ChessGame) MakeMove(move WSMoveUpdate) bool {
	if move.PlayerId == g.WhitePlayerId {
		if move.PlayerColor != "w" {
			fmt.Println("Expected player color to be white")
			return false
		}
	} else if move.PlayerId == g.BlackPlayerId {
		if move.PlayerColor != "b" {
			fmt.Println("Expected player color to be black")
			return false
		}
	} else {
		fmt.Println("Invalid player id")
		return false
	}

	color := strings.ToLower(g.GameState.Position().Turn().String())
	if color != move.PlayerColor {
		fmt.Println(color)
		return false
	}

	err := g.GameState.MoveStr(move.Move)
	if err != nil {
		fmt.Println("Invalid Move")
		return false
	}
	move.FEN = g.GameState.Position().String()

	fmt.Println("Broadcasting move")
	g.BroadcastUpdate(move)

	/* Check if game has ended - if so send a follow-up update */
	if g.GameState.Outcome() != chess.NoOutcome {
		resultUpdate := WSGameResultUpdate{
			Result: g.GameState.Outcome().String(),
			FEN:    g.GameState.Position().String(),
		}
		g.BroadcastUpdate(resultUpdate)
	}
	return true
}

func (g *ChessGames) AddNewGame() uint64 {
	/* Because we just have one Matchmaking goroutine calling this function we don't need to worry about
	 * synchronization yet
	 */
	gameId := g.NextAvailGameId
	g.NextAvailGameId += 1
	g.Games[gameId] = ChessGame{
		GameState:         chess.NewGame(),
		GameId:            gameId,
		WhitePlayerId:     g.NextAvailPlayerId,
		BlackPlayerId:     g.NextAvailPlayerId + 1,
		WhitePlayerStream: make(chan WSGameEvent, 10),
		BlackPlayerStream: make(chan WSGameEvent, 10),
		SpectatorStreams:  make([]chan WSGameEvent, 0),
	}
	g.NextAvailPlayerId += 2
	return gameId
}
