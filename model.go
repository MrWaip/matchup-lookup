package main

type Seed struct {
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
	Region   string `json:"region"`
	Source   string `json:"source"`
}

type Account struct {
	PUUID    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

type LeagueEntry struct {
	QueueType    string `json:"queueType"`
	Tier         string `json:"tier"`
	Rank         string `json:"rank"`
	LeaguePoints int    `json:"leaguePoints"`
}

type Match struct {
	Metadata struct {
		MatchID string `json:"matchId"`
	} `json:"metadata"`
	Info struct {
		GameCreation int64         `json:"gameCreation"`
		GameDuration int64         `json:"gameDuration"`
		GameVersion  string        `json:"gameVersion"`
		QueueID      int           `json:"queueId"`
		PlatformID   string        `json:"platformId"`
		Participants []Participant `json:"participants"`
	} `json:"info"`
}

type Participant struct {
	PUUID                string `json:"puuid"`
	RiotIDGameName       string `json:"riotIdGameName"`
	RiotIDTagline        string `json:"riotIdTagline"`
	ChampionName         string `json:"championName"`
	TeamPosition         string `json:"teamPosition"`
	IndividualPosition   string `json:"individualPosition"`
	Lane                 string `json:"lane"`
	TeamID               int    `json:"teamId"`
	Win                  bool   `json:"win"`
	Kills                int    `json:"kills"`
	Deaths               int    `json:"deaths"`
	Assists              int    `json:"assists"`
	TotalMinionsKilled   int    `json:"totalMinionsKilled"`
	NeutralMinionsKilled int    `json:"neutralMinionsKilled"`
	GoldEarned           int    `json:"goldEarned"`
	Item0                int    `json:"item0"`
	Item1                int    `json:"item1"`
	Item2                int    `json:"item2"`
	Item3                int    `json:"item3"`
	Item4                int    `json:"item4"`
	Item5                int    `json:"item5"`
	Item6                int    `json:"item6"`
}

type Player struct {
	PUUID    string
	GameName string
	TagLine  string
	Region   string
	Tier     string
	Division string
	LP       int
}
