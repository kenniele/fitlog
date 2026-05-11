package whoop

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCycleDTO_ToDomain(t *testing.T) {
	body := []byte(`{
        "id": 12345,
        "start": "2026-05-11T03:00:00.000Z",
        "end": "2026-05-12T03:00:00.000Z",
        "timezone_offset": "+03:00",
        "score_state": "SCORED",
        "score": {"strain": 13.6, "kilojoule": 9000, "average_heart_rate": 66, "max_heart_rate": 156}
    }`)
	var dto cycleDTO
	require.NoError(t, json.Unmarshal(body, &dto))

	out := dto.toDomain()
	require.Equal(t, int64(12345), out.ID)
	require.Equal(t, 13.6, out.Strain)
	require.Equal(t, 66.0, out.AvgHR)
	require.Equal(t, "+03:00", out.TZOffset)
	require.NotNil(t, out.End)
}

func TestCycleDTO_InProgress(t *testing.T) {
	body := []byte(`{"id":1,"start":"2026-05-11T03:00:00.000Z","end":null,"score_state":"PENDING_SCORE"}`)
	var dto cycleDTO
	require.NoError(t, json.Unmarshal(body, &dto))
	out := dto.toDomain()
	require.Nil(t, out.End)
	require.Equal(t, 0.0, out.Strain, "missing score → zero strain, not panic")
}

func TestRecoveryDTO_ToDomain(t *testing.T) {
	body := []byte(`{
        "cycle_id": 12345,
        "sleep_id": "sleep-uuid",
        "score": {
            "user_calibrating": false,
            "recovery_score": 72,
            "resting_heart_rate": 53,
            "hrv_rmssd_milli": 73,
            "spo2_percentage": 97.1,
            "skin_temp_celsius": 33.5
        }
    }`)
	var dto recoveryDTO
	require.NoError(t, json.Unmarshal(body, &dto))

	out := dto.toDomain()
	require.Equal(t, int64(12345), out.CycleID)
	require.Equal(t, 72.0, out.Score)
	require.Equal(t, 73.0, out.HRVMilli)
	require.Equal(t, 53.0, out.RestingHR)
	require.NotNil(t, out.SkinTempC)
	require.Equal(t, 33.5, *out.SkinTempC)
}

func TestRecoveryDTO_NoScore(t *testing.T) {
	body := []byte(`{"cycle_id":1,"sleep_id":"x","user_calibrating":true}`)
	var dto recoveryDTO
	require.NoError(t, json.Unmarshal(body, &dto))
	out := dto.toDomain()
	require.True(t, out.UserCalibrating)
	require.Equal(t, 0.0, out.Score)
	require.Nil(t, out.SkinTempC)
}

func TestSleepDTO_ToDomain(t *testing.T) {
	body := []byte(`{
        "id": "sleep-uuid",
        "start": "2026-05-11T00:00:00.000Z",
        "end": "2026-05-11T07:49:00.000Z",
        "nap": false,
        "score_state": "SCORED",
        "score": {
            "stage_summary": {
                "total_in_bed_time_milli": 28140000,
                "total_awake_time_milli": 600000,
                "total_light_sleep_time_milli": 15060000,
                "total_slow_wave_sleep_time_milli": 8280000,
                "total_rem_sleep_time_milli": 8940000,
                "sleep_cycle_count": 5,
                "disturbance_count": 3
            },
            "sleep_needed": {
                "baseline_milli": 28000000,
                "need_from_sleep_debt_milli": 0,
                "need_from_recent_strain_milli": 1000000,
                "need_from_recent_nap_milli": 0
            },
            "respiratory_rate": 14.5,
            "sleep_performance_percentage": 92,
            "sleep_consistency_percentage": 85,
            "sleep_efficiency_percentage": 92
        }
    }`)
	var dto sleepDTO
	require.NoError(t, json.Unmarshal(body, &dto))

	out := dto.toDomain()
	require.Equal(t, "sleep-uuid", out.ExternalID)
	require.False(t, out.IsNap)
	require.Equal(t, 92.0, out.SleepPerformancePct)
	require.Equal(t, int64(8940000), out.Stages.REMMs)
	require.Equal(t, 5, out.SleepCycleCount)
	require.Equal(t, int64(28000000), out.SleepNeed.BaselineMs)
}

func TestWorkoutDTO_ToDomain(t *testing.T) {
	body := []byte(`{
        "id": "wo-uuid",
        "sport_id": 59,
        "sport_name": "powerlifting",
        "start": "2026-05-11T11:10:00.000Z",
        "end": "2026-05-11T12:24:00.000Z",
        "timezone_offset": "+03:00",
        "score_state": "SCORED",
        "score": {
            "strain": 12.6,
            "average_heart_rate": 110,
            "max_heart_rate": 156,
            "kilojoule": 1500,
            "percent_recorded": 100,
            "distance_meter": null,
            "altitude_gain_meter": null,
            "zone_durations": {
                "zone_zero_milli": 0,
                "zone_one_milli": 1200000,
                "zone_two_milli": 2400000,
                "zone_three_milli": 600000,
                "zone_four_milli": 200000,
                "zone_five_milli": 0
            }
        }
    }`)
	var dto workoutDTO
	require.NoError(t, json.Unmarshal(body, &dto))

	out := dto.toDomain()
	require.Equal(t, 59, out.SportID)
	require.Equal(t, "Powerlifting", out.SportName, "local map should normalize the API's lowercase name")
	require.Equal(t, 12.6, out.Strain)
	require.Equal(t, 156.0, out.MaxHR)
	require.Equal(t, int64(2400000), out.ZoneDurations.Zone2)
	require.Nil(t, out.DistanceM)
}

func TestWorkoutDTO_UnknownSportID(t *testing.T) {
	body := []byte(`{"id":"x","sport_id":9999,"sport_name":"Future Sport","start":"2026-05-11T00:00:00Z","end":"2026-05-11T01:00:00Z"}`)
	var dto workoutDTO
	require.NoError(t, json.Unmarshal(body, &dto))
	out := dto.toDomain()
	require.Equal(t, "Future Sport", out.SportName)
}
