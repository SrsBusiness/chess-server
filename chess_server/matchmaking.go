package chess_server

import "math/rand"

type MatchRequest struct {
	Response chan<- MatchFoundResponse
}

/* Match found results */
type MatchFoundResponse struct {
	T           string `json:"type" xml:"type"`
	GameId      uint64 `json:"game_id" xml:"game_id"`
	PlayerId    uint64 `json:"player_id" xml:"player_id"`
	PlayerColor string `json:"player_color" xml:"player_color"`
}

type MatchMakingController struct {
	MatchRequests chan MatchRequest
	Games         *ChessGamesController
}

func (m *MatchMakingController) FindMatch(response chan<- MatchFoundResponse) {
	request := MatchRequest{
		Response: response,
	}
	m.MatchRequests <- request
}

func (m *MatchMakingController) Run() {
	for {
		r1 := <-m.MatchRequests
		r2 := <-m.MatchRequests

		game := m.Games.AddNewGame()
		gameId := game.GameId

		/* Randomly assign colors */
		var r1MatchFound MatchFoundResponse
		var r2MatchFound MatchFoundResponse
		if rand.Uint32()%2 == 0 {
			r1MatchFound = MatchFoundResponse{
				T:           "match_found",
				GameId:      gameId,
				PlayerId:    game.WhitePlayerId,
				PlayerColor: "w",
			}
			r2MatchFound = MatchFoundResponse{
				T:           "match_found",
				GameId:      gameId,
				PlayerId:    game.BlackPlayerId,
				PlayerColor: "b",
			}
		} else {
			r1MatchFound = MatchFoundResponse{
				T:           "match_found",
				GameId:      gameId,
				PlayerId:    game.BlackPlayerId,
				PlayerColor: "b",
			}
			r2MatchFound = MatchFoundResponse{
				T:           "match_found",
				GameId:      gameId,
				PlayerId:    game.WhitePlayerId,
				PlayerColor: "w",
			}
		}
		r1.Response <- r1MatchFound
		r2.Response <- r2MatchFound

	}
}

func (m *MatchMakingController) Init(g *ChessGamesController) {
	m.MatchRequests = make(chan MatchRequest)
	m.Games = g
}
