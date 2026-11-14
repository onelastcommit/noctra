package agent

import "strings"

type ModelPrices struct {
	InputPerMTok      float64
	OutputPerMTok     float64
	CacheWritePerMTok float64
	CacheReadPerMTok  float64
}

type modelPriceEntry struct {
	match  string
	prices ModelPrices
}

var modelPriceTable = []modelPriceEntry{
	{"fable-5-2", ModelPrices{InputPerMTok: 8, OutputPerMTok: 40, CacheWritePerMTok: 10, CacheReadPerMTok: 0.20}},
	{"mythos-5-2", ModelPrices{InputPerMTok: 8, OutputPerMTok: 40, CacheWritePerMTok: 10, CacheReadPerMTok: 0.20}},
	{"fable", ModelPrices{InputPerMTok: 10, OutputPerMTok: 50, CacheWritePerMTok: 12.50, CacheReadPerMTok: 0.25}},
	{"mythos", ModelPrices{InputPerMTok: 10, OutputPerMTok: 50, CacheWritePerMTok: 12.50, CacheReadPerMTok: 0.25}},
	{"opus-5-5", ModelPrices{InputPerMTok: 4, OutputPerMTok: 20, CacheWritePerMTok: 5, CacheReadPerMTok: 0.20}},
	{"opus", ModelPrices{InputPerMTok: 5, OutputPerMTok: 25, CacheWritePerMTok: 6.25, CacheReadPerMTok: 0.50}},
	{"sonnet-5-5", ModelPrices{InputPerMTok: 2, OutputPerMTok: 10, CacheWritePerMTok: 2.50, CacheReadPerMTok: 0.10}},
	{"sonnet-5", ModelPrices{InputPerMTok: 2, OutputPerMTok: 10, CacheWritePerMTok: 2.50, CacheReadPerMTok: 0.20}},
	{"sonnet", ModelPrices{InputPerMTok: 3, OutputPerMTok: 15, CacheWritePerMTok: 3.75, CacheReadPerMTok: 0.30}},
	{"haiku-5-5", ModelPrices{InputPerMTok: 0.10, OutputPerMTok: 0.50, CacheWritePerMTok: 0.125, CacheReadPerMTok: 0.01}},
	{"haiku", ModelPrices{InputPerMTok: 1, OutputPerMTok: 5, CacheWritePerMTok: 1.25, CacheReadPerMTok: 0.10}},
}

var fallbackModelPrices = ModelPrices{InputPerMTok: 3, OutputPerMTok: 15, CacheWritePerMTok: 3.75, CacheReadPerMTok: 0.30}

func PricesForModel(model string) ModelPrices {
	lowered := strings.ToLower(model)
	for _, entry := range modelPriceTable {
		if strings.Contains(lowered, entry.match) {
			return entry.prices
		}
	}
	return fallbackModelPrices
}

func (p ModelPrices) Estimate(inputTokens, outputTokens, cacheWriteTokens, cacheReadTokens int64) float64 {
	return float64(inputTokens)/1e6*p.InputPerMTok +
		float64(outputTokens)/1e6*p.OutputPerMTok +
		float64(cacheWriteTokens)/1e6*p.CacheWritePerMTok +
		float64(cacheReadTokens)/1e6*p.CacheReadPerMTok
}
