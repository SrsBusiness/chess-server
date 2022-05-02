package chess_server

type MatchRequest struct {
	Response *chan uint64
}

type MatchMaking struct {
	MatchRequests   chan MatchRequest
	NextAvailGameId uint64
}

func (m *MatchMaking) FindMatch(response *chan uint64) {
	request := MatchRequest{
		Response: response,
	}
	m.MatchRequests <- request
}

func (m *MatchMaking) Run() {
	for {
		r1 := <-m.MatchRequests
		r2 := <-m.MatchRequests
		gameId := m.NextAvailGameId
		m.NextAvailGameId += 1
		*r1.Response <- gameId
		*r2.Response <- gameId
	}
}

func (m *MatchMaking) Init() {
	m.MatchRequests = make(chan MatchRequest)
	m.NextAvailGameId = 0
}
