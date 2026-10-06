import type { NullableNumber, SourceStatus } from "@/lib/types";

export type CompactDay = {
  date: string;
  recovery_score: NullableNumber;
  hrv_ms: NullableNumber;
  resting_heart_rate_bpm: NullableNumber;
  daily_strain: NullableNumber;
  respiratory_rate: NullableNumber;
  spo2_percent: NullableNumber;
  skin_temperature_c: NullableNumber;
  sleep_seconds: NullableNumber;
  sleep_performance_percent: NullableNumber;
  sleep_efficiency_percent: NullableNumber;
  rem_seconds: NullableNumber;
  deep_seconds: NullableNumber;
  calories_kcal: NullableNumber;
  protein_g: NullableNumber;
  fat_g: NullableNumber;
  carbohydrates_g: NullableNumber;
};

export type CompactSnapshot = {
  version: number;
  generated_at: string;
  date: string;
  timezone: string;
  recovery: CompactDay | null;
  sleep: CompactDay | null;
  nutrition: CompactDay | null;
  body: {
    measured_at: string;
    weight_kg: NullableNumber;
    body_fat_percent: NullableNumber;
    skeletal_muscle_mass_kg: NullableNumber;
  } | null;
  training: {
    sessions: number;
    working_sets: number;
    repetitions: number;
    volume_kg: number;
    average_rir: NullableNumber;
  };
  next_workout: {
    id: number;
    title: string;
    scheduled_at: string | null;
  } | null;
  sources: SourceStatus[];
};
