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

type MatchMaking struct {
	MatchRequests chan MatchRequest
	Games         *ChessGames
}

func (m *MatchMaking) FindMatch(response chan<- MatchFoundResponse) {
	request := MatchRequest{
		Response: response,
	}
	m.MatchRequests <- request
}

func (m *MatchMaking) Run() {
	for {
		r1 := <-m.MatchRequests
		r2 := <-m.MatchRequests

		gameId := m.Games.AddNewGame()
		gameInfo := m.Games.Games[gameId]

		/* Randomly assign colors */
		var r1MatchFound MatchFoundResponse
		var r2MatchFound MatchFoundResponse
		if rand.Uint32()%2 == 0 {
			r1MatchFound = MatchFoundResponse{
				T:           "match_found",
				GameId:      gameId,
				PlayerId:    gameInfo.WhitePlayerId,
				PlayerColor: "white",
			}
			r2MatchFound = MatchFoundResponse{
				T:           "match_found",
				GameId:      gameId,
				PlayerId:    gameInfo.BlackPlayerId,
				PlayerColor: "black",
			}
		} else {
			r1MatchFound = MatchFoundResponse{
				T:           "match_found",
				GameId:      gameId,
				PlayerId:    gameInfo.BlackPlayerId,
				PlayerColor: "black",
			}
			r2MatchFound = MatchFoundResponse{
				T:           "match_found",
				GameId:      gameId,
				PlayerId:    gameInfo.WhitePlayerId,
				PlayerColor: "white",
			}
		}
		r1.Response <- r1MatchFound
		r2.Response <- r2MatchFound

	}
}

func (m *MatchMaking) Init(g *ChessGames) {
	m.MatchRequests = make(chan MatchRequest)
	m.Games = g
}
