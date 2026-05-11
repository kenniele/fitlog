package whoop

import "time"

// pageEnvelope is the common pagination envelope. The records field is
// re-typed per resource via Go generics on the client side.
type pageEnvelope[T any] struct {
	Records   []T    `json:"records"`
	NextToken string `json:"next_token,omitempty"`
}

// cycleDTO mirrors `/v2/cycle` records.
type cycleDTO struct {
	ID             int64      `json:"id"`
	Start          time.Time  `json:"start"`
	End            *time.Time `json:"end"`
	TimezoneOffset string     `json:"timezone_offset"`
	ScoreState     string     `json:"score_state"`
	Score          *struct {
		Strain           float64 `json:"strain"`
		Kilojoule        float64 `json:"kilojoule"`
		AverageHeartRate float64 `json:"average_heart_rate"`
		MaxHeartRate     float64 `json:"max_heart_rate"`
	} `json:"score"`
}

// recoveryDTO mirrors `/v2/recovery` records. Recovery has no own ID.
type recoveryDTO struct {
	CycleID         int64  `json:"cycle_id"`
	SleepID         string `json:"sleep_id"`
	UserCalibrating bool   `json:"user_calibrating"`
	ScoreState      string `json:"score_state"`
	Score           *struct {
		UserCalibrating  bool     `json:"user_calibrating"`
		RecoveryScore    float64  `json:"recovery_score"`
		RestingHeartRate float64  `json:"resting_heart_rate"`
		HRVRmssdMilli    float64  `json:"hrv_rmssd_milli"`
		SpO2Percentage   *float64 `json:"spo2_percentage"`
		SkinTempCelsius  *float64 `json:"skin_temp_celsius"`
	} `json:"score"`
}

// sleepDTO mirrors `/v2/activity/sleep` records.
type sleepDTO struct {
	ID             string    `json:"id"`
	Start          time.Time `json:"start"`
	End            time.Time `json:"end"`
	TimezoneOffset string    `json:"timezone_offset"`
	Nap            bool      `json:"nap"`
	ScoreState     string    `json:"score_state"`
	Score          *struct {
		StageSummary struct {
			TotalInBedTimeMilli         int64 `json:"total_in_bed_time_milli"`
			TotalAwakeTimeMilli         int64 `json:"total_awake_time_milli"`
			TotalLightSleepTimeMilli    int64 `json:"total_light_sleep_time_milli"`
			TotalSlowWaveSleepTimeMilli int64 `json:"total_slow_wave_sleep_time_milli"`
			TotalRemSleepTimeMilli      int64 `json:"total_rem_sleep_time_milli"`
			SleepCycleCount             int   `json:"sleep_cycle_count"`
			DisturbanceCount            int   `json:"disturbance_count"`
		} `json:"stage_summary"`
		SleepNeeded struct {
			BaselineMilli             int64 `json:"baseline_milli"`
			NeedFromSleepDebtMilli    int64 `json:"need_from_sleep_debt_milli"`
			NeedFromRecentStrainMilli int64 `json:"need_from_recent_strain_milli"`
			NeedFromRecentNapMilli    int64 `json:"need_from_recent_nap_milli"`
		} `json:"sleep_needed"`
		RespiratoryRate            float64 `json:"respiratory_rate"`
		SleepPerformancePercentage float64 `json:"sleep_performance_percentage"`
		SleepConsistencyPercentage float64 `json:"sleep_consistency_percentage"`
		SleepEfficiencyPercentage  float64 `json:"sleep_efficiency_percentage"`
	} `json:"score"`
}

// workoutDTO mirrors `/v2/activity/workout` records.
type workoutDTO struct {
	ID             string    `json:"id"`
	SportID        int       `json:"sport_id"`
	SportName      string    `json:"sport_name"`
	Start          time.Time `json:"start"`
	End            time.Time `json:"end"`
	TimezoneOffset string    `json:"timezone_offset"`
	ScoreState     string    `json:"score_state"`
	Score          *struct {
		Strain            float64  `json:"strain"`
		AverageHeartRate  float64  `json:"average_heart_rate"`
		MaxHeartRate      float64  `json:"max_heart_rate"`
		Kilojoule         float64  `json:"kilojoule"`
		PercentRecorded   float64  `json:"percent_recorded"`
		DistanceMeter     *float64 `json:"distance_meter"`
		AltitudeGainMeter *float64 `json:"altitude_gain_meter"`
		ZoneDurations     struct {
			ZoneZeroMilli  int64 `json:"zone_zero_milli"`
			ZoneOneMilli   int64 `json:"zone_one_milli"`
			ZoneTwoMilli   int64 `json:"zone_two_milli"`
			ZoneThreeMilli int64 `json:"zone_three_milli"`
			ZoneFourMilli  int64 `json:"zone_four_milli"`
			ZoneFiveMilli  int64 `json:"zone_five_milli"`
		} `json:"zone_durations"`
	} `json:"score"`
}
