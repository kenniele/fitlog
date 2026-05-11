package whoop

import "fitlog/internal/domain"

func (c *cycleDTO) toDomain() domain.Cycle {
	out := domain.Cycle{
		ID:         c.ID,
		Start:      c.Start,
		End:        c.End,
		TZOffset:   c.TimezoneOffset,
		ScoreState: c.ScoreState,
	}
	if c.Score != nil {
		out.Strain = c.Score.Strain
		out.AvgHR = c.Score.AverageHeartRate
		out.MaxHR = c.Score.MaxHeartRate
		out.Kilojoule = c.Score.Kilojoule
	}
	return out
}

func (r *recoveryDTO) toDomain() domain.Recovery {
	out := domain.Recovery{
		CycleID:         r.CycleID,
		SleepID:         r.SleepID,
		UserCalibrating: r.UserCalibrating,
	}
	if r.Score != nil {
		out.Score = r.Score.RecoveryScore
		out.HRVMilli = r.Score.HRVRmssdMilli
		out.RestingHR = r.Score.RestingHeartRate
		out.SkinTempC = r.Score.SkinTempCelsius
		out.SpO2Pct = r.Score.SpO2Percentage
		out.UserCalibrating = r.Score.UserCalibrating || r.UserCalibrating
	}
	return out
}

func (s *sleepDTO) toDomain() domain.Sleep {
	out := domain.Sleep{
		ExternalID: s.ID,
		Start:      s.Start,
		End:        s.End,
		IsNap:      s.Nap,
	}
	if s.Score != nil {
		out.SleepPerformancePct = s.Score.SleepPerformancePercentage
		out.SleepEfficiencyPct = s.Score.SleepEfficiencyPercentage
		out.SleepConsistencyPct = s.Score.SleepConsistencyPercentage
		out.RespiratoryRate = s.Score.RespiratoryRate
		out.Stages = domain.SleepStages{
			InBedMs: s.Score.StageSummary.TotalInBedTimeMilli,
			AwakeMs: s.Score.StageSummary.TotalAwakeTimeMilli,
			LightMs: s.Score.StageSummary.TotalLightSleepTimeMilli,
			SWSMs:   s.Score.StageSummary.TotalSlowWaveSleepTimeMilli,
			REMMs:   s.Score.StageSummary.TotalRemSleepTimeMilli,
		}
		out.DisturbanceCount = s.Score.StageSummary.DisturbanceCount
		out.SleepCycleCount = s.Score.StageSummary.SleepCycleCount
		out.SleepNeed = domain.SleepNeed{
			BaselineMs:   s.Score.SleepNeeded.BaselineMilli,
			FromNapMs:    s.Score.SleepNeeded.NeedFromRecentNapMilli,
			FromStrainMs: s.Score.SleepNeeded.NeedFromRecentStrainMilli,
			FromDebtMs:   s.Score.SleepNeeded.NeedFromSleepDebtMilli,
		}
	}
	return out
}

func (w *workoutDTO) toDomain() domain.Workout {
	out := domain.Workout{
		ExternalID: w.ID,
		SportID:    w.SportID,
		SportName:  resolveSportName(w.SportID, w.SportName),
		Start:      w.Start,
		End:        w.End,
		TZOffset:   w.TimezoneOffset,
	}
	if w.Score != nil {
		out.Strain = w.Score.Strain
		out.AvgHR = w.Score.AverageHeartRate
		out.MaxHR = w.Score.MaxHeartRate
		out.Kilojoule = w.Score.Kilojoule
		out.PercentRecorded = w.Score.PercentRecorded
		out.DistanceM = w.Score.DistanceMeter
		out.AltitudeGainM = w.Score.AltitudeGainMeter
		out.ZoneDurations = domain.ZoneDurations{
			Zone0: w.Score.ZoneDurations.ZoneZeroMilli,
			Zone1: w.Score.ZoneDurations.ZoneOneMilli,
			Zone2: w.Score.ZoneDurations.ZoneTwoMilli,
			Zone3: w.Score.ZoneDurations.ZoneThreeMilli,
			Zone4: w.Score.ZoneDurations.ZoneFourMilli,
			Zone5: w.Score.ZoneDurations.ZoneFiveMilli,
		}
	}
	return out
}
