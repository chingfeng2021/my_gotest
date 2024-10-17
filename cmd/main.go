package main

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"testgraph/mymodels"
	"time"
)

func QuerySingleUser(endpoint, accountID2 string, stageName string) {
	initSnaps, _ := mymodels.QueryUserInitLiquiditySnaps(endpoint, accountID2)
	snaps, _ := mymodels.QueryUserLiquiditySnaps(endpoint, accountID2, stageName, "cmd/config.json")
	fmt.Println("snaps Length is", len(snaps))
	point := mymodels.GetSnapshotMainPoint(time.Now().Unix(), initSnaps, snaps)
	// 格式化字符串并输出
	fmt.Printf("Init Point: %s, Total Point: %.2f\n", initSnaps.Account.BasePoints, point)

}

// 将字符串数组转换为 float64 数组
func convertToFloat64Array(strArr []string) ([]float64, error) {
	floatArr := make([]float64, len(strArr))
	for i, s := range strArr {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, err
		}
		floatArr[i] = f
	}
	return floatArr, nil
}

// 计算加权和
func calculateWeightedSum(lps, usd []float64) float64 {
	sum := 0.0
	for i := range lps {
		sum += lps[i] * usd[i]
	}
	return sum
}

// 计算评分
func calculateScore(snap mymodels.UserLiquiditySnap, currentTimestamp int64) (float64, error) {
	timeDiff := float64(currentTimestamp - snap.Timestamp)

	lps, err := convertToFloat64Array(snap.Lps)
	if err != nil {
		return 0, err
	}

	usd, err := convertToFloat64Array(snap.DerivedUSDs)
	if err != nil {
		return 0, err
	}

	weightedSum := calculateWeightedSum(lps, usd)

	basePoints, err := strconv.ParseFloat(snap.BasePoints, 64)
	if err != nil {
		return 0, err
	}

	return (basePoints + timeDiff*weightedSum) / 8640000, nil
}

// 按评分排序
func sortSnapsByScore(snaps []mymodels.UserLiquiditySnap, currentTimestamp int64, ascending bool) {
	sort.Slice(snaps, func(i, j int) bool {
		scoreI, _ := calculateScore(snaps[i], currentTimestamp)
		scoreJ, _ := calculateScore(snaps[j], currentTimestamp)
		if ascending {
			return scoreI < scoreJ // 顺序
		}
		return scoreI > scoreJ // 逆序
	})
}

// 按加权和排序
func sortSnapsByWeightedSum(snaps []mymodels.UserLiquiditySnap, ascending bool) {
	sort.Slice(snaps, func(i, j int) bool {
		lpsI, _ := convertToFloat64Array(snaps[i].Lps)
		usdI, _ := convertToFloat64Array(snaps[i].DerivedUSDs)
		sumI := calculateWeightedSum(lpsI, usdI)

		lpsJ, _ := convertToFloat64Array(snaps[j].Lps)
		usdJ, _ := convertToFloat64Array(snaps[j].DerivedUSDs)
		sumJ := calculateWeightedSum(lpsJ, usdJ)

		if ascending {
			return sumI < sumJ // 顺序
		}
		return sumI > sumJ // 逆序
	})
}

func QueryTotalScore(endpoint, stageName string) float64 {
	allUser, err := mymodels.QueryAllLiquiditySnapsForStage(endpoint, stageName, "cmd/config.json")
	if err != nil {
		return 0
	}

	for _, snap := range allUser {
		row := []string{
			snap.ID,
			strconv.FormatInt(snap.Timestamp, 10),
			snap.BasePoints,
			strings.Join(snap.Lps, "-"),
		}
		// 使用 %s 格式化输出，包含换行符
		fmt.Printf("%s %s %s %s\n", row[0], row[1], row[2], row[3])
	}

	maxSnaps := mymodels.GetMaxTimestampSnaps(allUser, false)

	fmt.Printf("聚合之后的数据集\n")
	for _, snap := range maxSnaps {
		lpsStr := strings.Join(snap.Lps, ", ")
		fmt.Printf("User: %s | Timestamp: %d | BasePoints: %s | Lps: [%s]\n",
			snap.ID[:42], // 打印用户地址
			snap.Timestamp,
			snap.BasePoints,
			lpsStr,
		)
	}
	currentTimestamp := time.Now().Unix()
	fmt.Printf("进行链下时间累加\n")
	// 计算每个用户的积分
	totalScore := 0.0
	for _, maxSnap := range maxSnaps {
		score := mymodels.CalculateScore(currentTimestamp, maxSnap)
		totalScore += score
		fmt.Printf("User: %s, Score: %f\n", maxSnap.ID, score)
	}
	// 修改打印语句，添加格式化占位符
	fmt.Printf("当前所有总积分：%.2f\n", totalScore) // 格式化为小数点后两位

	//// // DT: 查询阶段总积分， 把所有快照积分求和 + 首段:(lp * 平均价格 * 时间) +  尾端: (lp * 平均价格 * 时间)

	return totalScore
}

// sortBySwapUSD 根据 UserSwap 的 SwapUSD 进行排序
func sortBySwapUSD(snaps []mymodels.UserLiquiditySnap, swaps []mymodels.UserSwap, ascending bool) {
	// 构建 ID -> UserSwap 的映射，加快查找速度
	swapsMap := make(map[string]string)
	for _, swap := range swaps {
		swapsMap[swap.ID] = swap.SwapUSD
	}

	// 排序逻辑
	sort.Slice(snaps, func(i, j int) bool {
		swapUSDi := getSwapUSD(swapsMap, snaps[i].ID)
		swapUSDj := getSwapUSD(swapsMap, snaps[j].ID)

		if ascending {
			return swapUSDi < swapUSDj
		}
		return swapUSDi > swapUSDj
	})
}

// getSwapUSD 获取 SwapUSD 的浮点值
func getSwapUSD(swapsMap map[string]string, id string) float64 {
	swapUSDStr, exists := swapsMap[id]
	if !exists {
		log.Fatalf("未找到匹配的 UserSwap: ID = %s", id)
	}
	swapUSD, err := strconv.ParseFloat(swapUSDStr, 64)
	if err != nil {
		log.Fatalf("转换 SwapUSD 失败: %v", err)
	}
	return swapUSD
}

// printSnaps 打印排序结果
func printSnaps(snaps []mymodels.UserLiquiditySnap) {
	for _, snap := range snaps {
		fmt.Printf("ID: %s, Timestamp: %d\n", snap.ID, snap.Timestamp)
	}
}
func RankList(endpoint, stageName, method, asc string) (map[string]map[string]interface{}, error) {
	allUser, err := mymodels.QueryAllLiquiditySnapsForStage(endpoint, stageName, "cmd/config.json")
	if err != nil {
		return nil, err
	}
	//for _, snap := range allUser {
	//	row := []string{
	//		snap.ID,
	//		strconv.FormatInt(snap.Timestamp, 10),
	//		snap.BasePoints,
	//		strings.Join(snap.Lps, "-"),
	//	}
	//	// 使用 %s 格式化输出，包含换行符
	//	fmt.Printf("%s %s %s %s\n", row[0], row[1], row[2], row[3])
	//}
	maxSnaps := mymodels.GetMaxTimestampSnaps(allUser, true)
	ascending := asc == "1"
	switch method {
	case "score":
		sortSnapsByScore(maxSnaps, time.Now().Unix(), ascending)
	case "lp":
		sortSnapsByWeightedSum(maxSnaps, ascending)
	//case "swap":
	//	if err := sortSnapsBySwapUSD(maxSnaps, ascending); err != nil {
	//		return nil, err
	//	}
	default:
		return nil, errors.New("void method")
	}
	result := make(map[string]map[string]interface{})

	for key, snap := range maxSnaps {
		score, _ := calculateScore(snap, time.Now().Unix())

		data := map[string]interface{}{
			"id":        snap.ID,
			"score":     score,
			"timestamp": snap.Timestamp,
			"lp":        snap.Lps,
		}
		result[strconv.Itoa(key)] = data
		fmt.Printf("ID: %s, Score: %.2f, Rank: %d\n", snap.ID, score, snap.Rank)

	}

	return result, nil
}

func main() {
	endpoint := "https://api.goldsky.com/api/public/project_cm24cvlgu3nvw01ukay5gamhh/subgraphs/poap-subgraph/1.0.4/gn"

	// DT测试单元: 目前计算了时间差值，LP等值美元的总量
	// endpoint := "https://api.studio.thegraph.com/query/20815/artex-swap-lp/version/latest"

	// 查询用户的初始化快照 PASS
	// accountID := "0x17c1b4f4f28c84f4c2cca2ed672cd8ce9e590407"
	//initSnaps, err := mymodels.QueryUserInitLiquiditySnaps(endpoint, accountID)
	//if err != nil {
	//	fmt.Println("Query failed:", err)
	//	return
	//}
	//fmt.Println("initSnaps:", initSnaps)

	// 查询用户的阶段性积分：通过配置获取开始和结束时间，这里先把时间打桩
	// 先获取初始化分数快照，再获取用户阶段积分，主体计算：PASS
	// 计算单个用户得分
	// accountID2 := "0x17c1b4f4f28c84f4c2cca2ed672cd8ce9e590407"

	// QuerySingleUser(endpoint, accountID2, "Stage1")

	// 计算总分
	// QueryTotalScore(endpoint, "Stage1")

	// 输出排行榜
	list, err := RankList(endpoint, "Stage1", "score", "0")
	if err != nil {
		return
	}
	fmt.Printf("length is %d", len(list))
}
