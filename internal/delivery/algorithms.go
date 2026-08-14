package delivery

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

type BayesianInput struct {
	PriorAlpha float64 `json:"prior_alpha"`
	PriorBeta  float64 `json:"prior_beta"`
	Successes  int64   `json:"successes"`
	Trials     int64   `json:"trials"`
}

type BayesianResult struct {
	PosteriorAlpha float64 `json:"posterior_alpha"`
	PosteriorBeta  float64 `json:"posterior_beta"`
	Mean           float64 `json:"mean"`
	Variance       float64 `json:"variance"`
	CredibleLow95  float64 `json:"credible_low_95"`
	CredibleHigh95 float64 `json:"credible_high_95"`
	Method         string  `json:"method"`
}

func BetaBinomialUpdate(input BayesianInput) (BayesianResult, error) {
	if input.PriorAlpha <= 0 || input.PriorBeta <= 0 {
		return BayesianResult{}, errors.New("prior_alpha and prior_beta must be positive")
	}
	if !finite(input.PriorAlpha) || !finite(input.PriorBeta) {
		return BayesianResult{}, errors.New("prior values must be finite")
	}
	if input.Successes < 0 || input.Trials < 0 || input.Successes > input.Trials {
		return BayesianResult{}, errors.New("successes must be between zero and trials")
	}
	alpha := input.PriorAlpha + float64(input.Successes)
	beta := input.PriorBeta + float64(input.Trials-input.Successes)
	total := alpha + beta
	mean := alpha / total
	variance := alpha * beta / (total * total * (total + 1))
	delta := 1.959963984540054 * math.Sqrt(variance)
	return BayesianResult{
		PosteriorAlpha: alpha,
		PosteriorBeta:  beta,
		Mean:           mean,
		Variance:       variance,
		CredibleLow95:  math.Max(0, mean-delta),
		CredibleHigh95: math.Min(1, mean+delta),
		Method:         "beta-binomial-normal-approximation/v1",
	}, nil
}

type BudgetArm struct {
	Key               string  `json:"key"`
	MinFen            int64   `json:"min_fen"`
	MaxFen            int64   `json:"max_fen"`
	IncrementFen      int64   `json:"increment_fen,omitempty"`
	ExpectedValue     float64 `json:"expected_value"`
	Uncertainty       float64 `json:"uncertainty,omitempty"`
	RiskPenalty       float64 `json:"risk_penalty,omitempty"`
	MinimumSampleSize int64   `json:"minimum_sample_size,omitempty"`
	ObservedSamples   int64   `json:"observed_samples,omitempty"`
}

type BudgetOptimizationInput struct {
	TotalFen        int64       `json:"total_fen"`
	ExplorationRate float64     `json:"exploration_rate,omitempty"`
	Arms            []BudgetArm `json:"arms"`
}

type BudgetAllocation struct {
	Key       string  `json:"key"`
	AmountFen int64   `json:"amount_fen"`
	Score     float64 `json:"score"`
	Reason    string  `json:"reason"`
}

type BudgetOptimizationResult struct {
	TotalFen       int64              `json:"total_fen"`
	AllocatedFen   int64              `json:"allocated_fen"`
	UnallocatedFen int64              `json:"unallocated_fen"`
	Allocations    []BudgetAllocation `json:"allocations"`
	Method         string             `json:"method"`
	Executable     bool               `json:"executable"`
}

func OptimizeBudget(input BudgetOptimizationInput) (BudgetOptimizationResult, error) {
	if input.TotalFen <= 0 {
		return BudgetOptimizationResult{}, errors.New("total_fen must be positive")
	}
	if input.ExplorationRate < 0 || input.ExplorationRate > 1 {
		return BudgetOptimizationResult{}, errors.New("exploration_rate must be between zero and one")
	}
	if len(input.Arms) == 0 || len(input.Arms) > 200 {
		return BudgetOptimizationResult{}, errors.New("arms must contain between 1 and 200 values")
	}
	type workingArm struct {
		BudgetArm
		allocation int64
		score      float64
	}
	arms := make([]workingArm, len(input.Arms))
	keys := map[string]struct{}{}
	var minimum int64
	for index, arm := range input.Arms {
		if arm.Key == "" {
			return BudgetOptimizationResult{}, fmt.Errorf("arm %d key is required", index)
		}
		if _, exists := keys[arm.Key]; exists {
			return BudgetOptimizationResult{}, fmt.Errorf("arm key %q is duplicated", arm.Key)
		}
		keys[arm.Key] = struct{}{}
		if arm.MinFen < 0 || arm.MaxFen < arm.MinFen {
			return BudgetOptimizationResult{}, fmt.Errorf("arm %q has invalid min/max", arm.Key)
		}
		if arm.IncrementFen <= 0 {
			arm.IncrementFen = 100
		}
		if !finite(arm.ExpectedValue) || !finite(arm.Uncertainty) || !finite(arm.RiskPenalty) || arm.ExpectedValue < 0 || arm.Uncertainty < 0 || arm.RiskPenalty < 0 {
			return BudgetOptimizationResult{}, fmt.Errorf("arm %q scores cannot be negative", arm.Key)
		}
		samplePenalty := 0.0
		if arm.MinimumSampleSize > 0 && arm.ObservedSamples < arm.MinimumSampleSize {
			samplePenalty = 0.25 * (1 - float64(arm.ObservedSamples)/float64(arm.MinimumSampleSize))
		}
		score := arm.ExpectedValue + input.ExplorationRate*arm.Uncertainty - arm.RiskPenalty - samplePenalty
		arms[index] = workingArm{BudgetArm: arm, allocation: arm.MinFen, score: score}
		minimum += arm.MinFen
	}
	if minimum > input.TotalFen {
		return BudgetOptimizationResult{}, errors.New("sum of arm minimums exceeds total_fen")
	}
	remaining := input.TotalFen - minimum
	sort.SliceStable(arms, func(i, j int) bool {
		if arms[i].score == arms[j].score {
			return arms[i].Key < arms[j].Key
		}
		return arms[i].score > arms[j].score
	})
	for index := range arms {
		capacity := arms[index].MaxFen - arms[index].allocation
		if capacity <= 0 || remaining <= 0 {
			continue
		}
		allocation := minInt64(capacity, remaining)
		allocation -= allocation % arms[index].IncrementFen
		if allocation <= 0 {
			continue
		}
		arms[index].allocation += allocation
		remaining -= allocation
	}
	result := BudgetOptimizationResult{
		TotalFen: input.TotalFen, AllocatedFen: input.TotalFen - remaining,
		UnallocatedFen: remaining, Allocations: make([]BudgetAllocation, 0, len(arms)),
		Method: "constrained-greedy-marginal-value/v1", Executable: false,
	}
	for _, arm := range arms {
		result.Allocations = append(result.Allocations, BudgetAllocation{
			Key: arm.Key, AmountFen: arm.allocation, Score: arm.score,
			Reason: "expected_value + exploration*uncertainty - risk - sample_penalty",
		})
	}
	return result, nil
}

type BanditArm struct {
	Key          string  `json:"key"`
	Pulls        int64   `json:"pulls"`
	RewardSum    float64 `json:"reward_sum"`
	ContextScore float64 `json:"context_score,omitempty"`
}

type BanditShadowInput struct {
	Arms               []BanditArm `json:"arms"`
	ExplorationWeight  float64     `json:"exploration_weight,omitempty"`
	MinimumPullsPerArm int64       `json:"minimum_pulls_per_arm,omitempty"`
}

type BanditShadowResult struct {
	SelectedKey string             `json:"selected_key"`
	Scores      map[string]float64 `json:"scores"`
	Method      string             `json:"method"`
	ShadowOnly  bool               `json:"shadow_only"`
	Reason      string             `json:"reason"`
}

func BanditShadowSuggestion(input BanditShadowInput) (BanditShadowResult, error) {
	if len(input.Arms) < 2 || len(input.Arms) > 100 {
		return BanditShadowResult{}, errors.New("arms must contain between 2 and 100 values")
	}
	if input.ExplorationWeight == 0 {
		input.ExplorationWeight = 1
	}
	if input.ExplorationWeight < 0 || input.MinimumPullsPerArm < 0 {
		return BanditShadowResult{}, errors.New("exploration settings cannot be negative")
	}
	var totalPulls int64
	keys := map[string]struct{}{}
	for _, arm := range input.Arms {
		if arm.Key == "" || arm.Pulls < 0 || !finite(arm.RewardSum) || !finite(arm.ContextScore) {
			return BanditShadowResult{}, errors.New("each arm needs a key and non-negative pulls")
		}
		if _, exists := keys[arm.Key]; exists {
			return BanditShadowResult{}, fmt.Errorf("arm key %q is duplicated", arm.Key)
		}
		keys[arm.Key] = struct{}{}
		totalPulls += arm.Pulls
	}
	result := BanditShadowResult{Scores: map[string]float64{}, Method: "contextual-ucb-shadow/v1", ShadowOnly: true}
	bestScore := math.Inf(-1)
	for _, arm := range input.Arms {
		score := 1_000_000_000_000.0 + arm.ContextScore
		if arm.Pulls >= maxInt64(1, input.MinimumPullsPerArm) {
			mean := arm.RewardSum / float64(arm.Pulls)
			exploration := input.ExplorationWeight * math.Sqrt(2*math.Log(float64(maxInt64(totalPulls, 2)))/float64(arm.Pulls))
			score = mean + exploration + arm.ContextScore
		}
		result.Scores[arm.Key] = score
		if score > bestScore || score == bestScore && arm.Key < result.SelectedKey {
			bestScore = score
			result.SelectedKey = arm.Key
		}
	}
	result.Reason = "shadow recommendation only; activation, bids, and budgets are never changed by this endpoint"
	return result, nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func minInt64(values ...int64) int64 {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
