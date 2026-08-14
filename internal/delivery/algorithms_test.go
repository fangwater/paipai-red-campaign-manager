package delivery

import (
	"encoding/json"
	"math"
	"testing"
)

func TestBetaBinomialUpdate(t *testing.T) {
	result, err := BetaBinomialUpdate(BayesianInput{PriorAlpha: 1, PriorBeta: 1, Successes: 8, Trials: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.PosteriorAlpha != 9 || result.PosteriorBeta != 3 || math.Abs(result.Mean-0.75) > 1e-9 {
		t.Fatalf("unexpected posterior: %+v", result)
	}
	if result.CredibleLow95 < 0 || result.CredibleHigh95 > 1 || result.CredibleLow95 >= result.CredibleHigh95 {
		t.Fatalf("invalid credible interval: %+v", result)
	}
	if _, err := BetaBinomialUpdate(BayesianInput{PriorAlpha: math.NaN(), PriorBeta: 1}); err == nil {
		t.Fatal("BetaBinomialUpdate accepted NaN")
	}
}

func TestOptimizeBudgetHonorsCapsAndRanksByScore(t *testing.T) {
	result, err := OptimizeBudget(BudgetOptimizationInput{
		TotalFen: 30_000,
		Arms: []BudgetArm{
			{Key: "high", MinFen: 5_000, MaxFen: 20_000, IncrementFen: 1_000, ExpectedValue: 0.9},
			{Key: "low", MinFen: 5_000, MaxFen: 20_000, IncrementFen: 1_000, ExpectedValue: 0.3},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Executable || result.AllocatedFen != 30_000 || result.UnallocatedFen != 0 {
		t.Fatalf("unexpected optimizer result: %+v", result)
	}
	if result.Allocations[0].Key != "high" || result.Allocations[0].AmountFen != 20_000 || result.Allocations[1].AmountFen != 10_000 {
		t.Fatalf("allocation does not prioritize the higher score: %+v", result.Allocations)
	}
	if _, err := OptimizeBudget(BudgetOptimizationInput{TotalFen: 1, Arms: []BudgetArm{{Key: "bad", MaxFen: 1, ExpectedValue: math.Inf(1)}}}); err == nil {
		t.Fatal("OptimizeBudget accepted infinity")
	}
}

func TestBanditSuggestionIsFiniteShadowOnly(t *testing.T) {
	result, err := BanditShadowSuggestion(BanditShadowInput{
		MinimumPullsPerArm: 3,
		Arms: []BanditArm{
			{Key: "unseen", Pulls: 0},
			{Key: "known", Pulls: 10, RewardSum: 5},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.ShadowOnly || result.SelectedKey != "unseen" || math.IsInf(result.Scores["unseen"], 0) {
		t.Fatalf("unexpected bandit result: %+v", result)
	}
	if _, err := json.Marshal(result); err != nil {
		t.Fatalf("bandit result cannot be encoded as JSON: %v", err)
	}
}
