package mymodels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
)

// QueryUserLiquiditySnaps constructs the GraphQL query and retrieves user liquidity snapshots.
func QueryUserLiquiditySnaps(endpoint, accountID, stageName string, configFile string) ([]UserLiquiditySnap, error) {
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

	// Build the GraphQL query dynamically.
	query := fmt.Sprintf(`{
		userLiquiditySnaps(
			where: {timestamp_lte: %s, timestamp_gt: %s, account_: {id: "%s"}}
		) {
			timestamp
			id
			lps
			derivedUSDs
			account {
			  id
			  lps
			  basePoints
			}
		}
	}`, selectedStage["endTime"], selectedStage["startTime"], accountID)

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

func QueryUserInitLiquiditySnaps(endpoint, accountID string) (UserLiquiditySnap, error) {
	// 构建 GraphQL 查询语句。
	query := fmt.Sprintf(`
		query MyQuery {
			userLiquiditySnaps(
				where: { account: "%s", timestamp_gte: %d }
				first: 1
			) {
				timestamp
				id
				lps
				account {
					id
					lps
					basePoints
					derivedUSDs
				}
				basePoints
			}
		}`, accountID, 0)

	// 发送请求并获取响应体。
	body, err := sendGraphQLRequest(endpoint, query)
	if err != nil {
		return UserLiquiditySnap{}, err // 返回空的 UserLiquiditySnap
	}

	// 解析 GraphQL 响应。
	var gqlResp GraphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return UserLiquiditySnap{}, fmt.Errorf("query failed: decoding response: %w", err) // 返回空的 UserLiquiditySnap
	}

	if len(gqlResp.Data.UserLiquiditySnaps) > 0 {
		return gqlResp.Data.UserLiquiditySnaps[0], nil
	}
	return UserLiquiditySnap{}, nil // 返回空的 UserLiquiditySnap
}

// sendGraphQLRequest sends a GraphQL query and returns the response body.
func sendGraphQLRequest(endpoint string, query string) ([]byte, error) {
	// Prepare the payload for the GraphQL request.
	payload := map[string]string{"query": query}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare request: %w", err)
	}

	// Create the HTTP request.
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request using an HTTP client.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, nil
}

// GetSnapshotTimestamps retrieves the first and last timestamps from the snapshots.
func GetSnapshotMainPoint(currentTimestamp int64, initsnap UserLiquiditySnap, snaps []UserLiquiditySnap) float64 {
	if len(snaps) == 0 {
		return 0
	}
	start, _ := strconv.ParseFloat(snaps[0].Account.BasePoints, 64)
	end, _ := strconv.ParseFloat(snaps[len(snaps)-1].Account.BasePoints, 64)
	init, err := strconv.ParseFloat(initsnap.Account.BasePoints, 64)
	if err != nil {
		fmt.Printf("Error converting BasePoints to float: %v\n", err)
	} else {
		fmt.Printf("BasePoints as float: %f\n", init)
	}

	// 计算加权和
	weightedSum := calculateWeightedSum(snaps[len(snaps)-1].Lps, snaps[len(snaps)-1].DerivedUSDs)

	// 计算时间差
	timeDifference := currentTimestamp - snaps[len(snaps)-1].Timestamp
	// 在输出积分的部分中
	fmt.Printf("时间差: %d, 加权结果: %.2f\n", timeDifference, weightedSum)

	fmt.Println("point info:", snaps[0].Account.BasePoints, end)
	return (end - start + init + float64(timeDifference)*weightedSum) / 8640000
}

// GetSnapshotTimestamps retrieves the first and last timestamps from the snapshots.
func GetSnapshotTimestamps(snaps []UserLiquiditySnap) (int64, int64) {
	if len(snaps) == 0 {
		return 0, 0
	}
	return snaps[0].Timestamp, snaps[len(snaps)-1].Timestamp
}

// CalculateTotalTimeDifference computes the sum of two time differences:
// 1. Activity start time to first snapshot time.
// 2. Last snapshot time to the current time.
func CalculateTotalTimeDifference(activityStart, firstSnapshot, lastSnapshot int64) time.Duration {

	startToFirstDiff := time.Unix(firstSnapshot, 0).Sub(time.Unix(activityStart, 0))

	lastToCurrentDiff := time.Now().Sub(time.Unix(lastSnapshot, 0))

	return startToFirstDiff + lastToCurrentDiff
}

//func CalculateRemainPoint(activityStart, firstSnapshot, lastSnapshot int64) time.Duration {
//
//	startToFirstDiff := time.Unix(firstSnapshot, 0).Sub(time.Unix(activityStart, 0))
//	startToFirstDiffPoint = startToFirstDiff *
//
//	lastToCurrentDiff := time.Now().Sub(time.Unix(lastSnapshot, 0))
//
//	return startToFirstDiff + lastToCurrentDiff
//}

// SumInnerLps

// SumLps calculates the sum of all derivedUSDs from the snapshots.
func SumLps(snaps []UserLiquiditySnap) (float64, error) {
	var total float64
	for _, snap := range snaps {
		for _, derivedUSD := range snap.Account.Lps {
			value, err := strconv.ParseFloat(derivedUSD, 64)
			if err != nil {
				return 0, fmt.Errorf("conversion failed: %w", err)
			}
			total += value
		}
	}
	return total, nil
}
