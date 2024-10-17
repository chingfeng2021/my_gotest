package mymodels

// GraphQLResponse is used to parse the GraphQL response.
type GraphQLResponse struct {
	Data struct {
		UserLiquiditySnaps []UserLiquiditySnap `json:"userLiquiditySnaps"`
	} `json:"data"`
}

// Stage holds the details of each activity stage.
type Stage struct {
	Name             string  `json:"name"`
	StartTime        string  `json:"startTime"`
	EndTime          string  `json:"endTime"`
	EarningRate      float64 `json:"earningRate"`
	TotalPointCap    int     `json:"totalPointCap"`
	SingleAddressCap int     `json:"singleAddressCap"`
}

// UserLiquiditySnap represents a user's liquidity snapshot.
type UserLiquiditySnap struct {
	Timestamp   int64    `json:"timestamp"`
	Lps         []string `json:"lps"`
	BasePoints  string   `json:"basePoints"`
	DerivedUSDs []string `json:"derivedUSDs"`
	ID          string   `json:"id"`
	Account     Account  `json:"account"`
	Rank        int64    `json:"rank"`
}

// Account holds user account information.
type Account struct {
	ID         string   `json:"id"`
	Lps        []string `json:"derivedUSDs"`
	BasePoints string   `json:"basePoints"`
}

// UserSwap 结构体
type UserSwap struct {
	SwapUSD string `json:"swapUSD"`
	Swap    string `json:"swap"`
	ID      string `json:"id"`
}
