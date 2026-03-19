### Experimental feature stages are already validated during decode

**Location:** `experimental.go:27` — `ExperimentalFeatureStage.UnmarshalJSON`

**Reason:** The current code already defines `ExperimentalFeatureStage.UnmarshalJSON`, backed by the
closed `validExperimentalFeatureStages` set in `experimental.go:19-25`. `ExperimentalFeature` uses
that type for its `Stage` field, so invalid values are rejected during `json.Unmarshal`. The real
decode path is also covered by `experimental_test.go:159-177`.
