package mymodels

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"
)

// LoadConfig loads configuration from the specified JSON file.
func LoadConfig(filename string) (map[string]interface{}, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := make(map[string]interface{})
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// GetStage retrieves the specified stage from the configuration.
func GetStage(config map[string]interface{}, stageName string) (map[string]interface{}, error) {
	stages, ok := config["stages"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid type for 'stages'")
	}

	for _, stage := range stages {
		stageMap, ok := stage.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid stage type")
		}
		if stageMap["name"] == stageName {
			return stageMap, nil
		}
	}

	return nil, fmt.Errorf("stage '%s' not found", stageName)
}

// QueryLiquiditySnapsForStage 查询指定阶段的流动性快照
func QueryAllLiquiditySnapsForStage(endpoint, stageName string, configFile string) ([]UserLiquiditySnap, error) {
	// 加载配置
	config, err := LoadConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}
	// Retrieve the selected stage
	selectedStage, err := GetStage(config, stageName)
	if err != nil {
		log.Fatalf("Error retrieving stage: %v", err)
	}
	fmt.Println(selectedStage["startTime"], selectedStage["endTime"])

	// 获取 selectedStage 的开始和结束时间，并确保是字符串类型
	startTime, ok1 := selectedStage["startTime"].(string)
	endTime, ok2 := selectedStage["endTime"].(string)

	if !ok1 || !ok2 {
		log.Fatalf("Invalid start or end time in selectedStage: %v, %v", startTime, endTime)
	}

	query := fmt.Sprintf(`{
		userLiquiditySnaps(
			where: {timestamp_lte: %s, timestamp_gt: %s}
		) {
			timestamp
			id
			lps
			basePoints
			derivedUSDs
			account {
			  id
			  lps
			  basePoints
			}
		}
	}`, endTime, startTime)

	// Send the request and get the response body.
	body, err := sendGraphQLRequest(endpoint, query)
	if err != nil {
		return nil, err
	}

	// Parse the GraphQL response.
	var gqlResp GraphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("query failed: decoding response: %w", err)
	}
	return gqlResp.Data.UserLiquiditySnaps, nil
}

// 查询积分总值

func GetMaxTimestampSnaps(allUser []UserLiquiditySnap, sort bool) []UserLiquiditySnap {
	// 用于存储每个用户的最大时间戳事件片

	userMaxSnap := make(map[string]UserLiquiditySnap)

	for _, snap := range allUser {
		// 提取用户地址（ID 中的前半部分）
		userID := snap.ID[:42]

		// 如果当前用户没有记录，或者发现更大的时间戳，则更新
		if existingSnap, found := userMaxSnap[userID]; !found || snap.Timestamp > existingSnap.Timestamp {
			userMaxSnap[userID] = snap
		}
	}

	// 将 map 转为 slice 以便返回
	maxSnaps := make([]UserLiquiditySnap, 0, len(userMaxSnap))
	for _, snap := range userMaxSnap {
		snap.Rank = 0
		maxSnaps = append(maxSnaps, snap)
	}

	if sort == true {
		sortSnapsByScore(maxSnaps, time.Now().Unix(), false)
	}
	sortMaxSnaps := make([]UserLiquiditySnap, 0, len(maxSnaps))
	index := 1
	for _, snap := range maxSnaps {
		snap.Rank = int64(index)
		index++
		sortMaxSnaps = append(sortMaxSnaps, snap)
	}

	return sortMaxSnaps
}

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
func calculateWeightedSumfloat(lps, usd []float64) float64 {
	sum := 0.0
	for i := range lps {
		sum += lps[i] * usd[i]
	}
	return sum
}

// 计算评分
func calculateScore(snap UserLiquiditySnap, currentTimestamp int64) (float64, error) {
	timeDiff := float64(currentTimestamp - snap.Timestamp)

	lps, err := convertToFloat64Array(snap.Lps)
	if err != nil {
		return 0, err
	}

	usd, err := convertToFloat64Array(snap.DerivedUSDs)
	if err != nil {
		return 0, err
	}

	weightedSum := calculateWeightedSumfloat(lps, usd)

	basePoints, err := strconv.ParseFloat(snap.BasePoints, 64)
	if err != nil {
		return 0, err
	}

	return (basePoints + timeDiff*weightedSum) / 8640000, nil
}

// 按评分排序
func sortSnapsByScore(snaps []UserLiquiditySnap, currentTimestamp int64, ascending bool) {
	sort.Slice(snaps, func(i, j int) bool {
		scoreI, _ := calculateScore(snaps[i], currentTimestamp)
		scoreJ, _ := calculateScore(snaps[j], currentTimestamp)
		if ascending {
			return scoreI < scoreJ // 顺序
		}
		return scoreI > scoreJ // 逆序
	})
}

// 计算 LP 和 DerivedUSDs 的加权和
func calculateWeightedSum(lps, derivedUSDs []string) float64 {
	totalValue := 0.0

	// 确保数组长度一致
	if len(lps) != len(derivedUSDs) {
		return totalValue
	}

	// 计算每个 LP 和对应的 DerivedUSDs 的乘积
	for i := 0; i < len(lps); i++ {
		lpValue, err1 := strconv.ParseFloat(lps[i], 64)
		derivedValue, err2 := strconv.ParseFloat(derivedUSDs[i], 64)
		if err1 == nil && err2 == nil {
			totalValue += lpValue * derivedValue
		}
	}
	return totalValue
}

// 计算积分
func CalculateScore(currentTimestamp int64, maxSnap UserLiquiditySnap) float64 {
	// 转换 BasePoints 值
	basePointsValue, err := strconv.ParseFloat(maxSnap.BasePoints, 64)
	if err != nil {
		return 0 // 返回 0 或者其他默认值
	}

	// 计算加权和
	weightedSum := calculateWeightedSum(maxSnap.Lps, maxSnap.DerivedUSDs)

	// 计算时间差
	timeDifference := currentTimestamp - maxSnap.Timestamp
	// 在输出积分的部分中
	fmt.Printf("时间差: %d, 加权结果: %.2f\n", timeDifference, weightedSum)

	// 计算积分
	score := (basePointsValue + float64(timeDifference)*weightedSum) / 8640000
	return score
}
