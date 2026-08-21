export type NullableNumber = number | null;
export type SourceName = "whoop" | "fatsecret" | "manual" | "csv" | "json" | string;

export type SeriesPoint = {
  date: string;
  value?: NullableNumber;
  [key: string]: string | number | boolean | null | undefined;
};

export type Metric = {
  key?: string;
  label?: string;
  value?: string | number | null;
  unit?: string | null;
  delta?: NullableNumber;
  context?: string | null;
  status?: "good" | "warning" | "critical" | "neutral" | null;
  series?: SeriesPoint[] | null;
};

export type SourceStatus = {
  source: SourceName;
  label?: string;
  connected?: boolean;
  status?: string | null;
  last_synced_at?: string | null;
  last_error?: string | null;
};

export type Highlight = {
  id?: string | number;
  type?: "positive" | "warning" | "critical" | "neutral" | string;
  title: string;
  description?: string | null;
  date?: string | null;
  rule?: string | null;
};

export type WorkoutSet = {
  id?: number | string;
  type?: "warmup" | "working" | "drop" | string;
  weight_kg?: NullableNumber;
  actual_weight_kg?: NullableNumber;
  reps?: NullableNumber;
  actual_reps?: NullableNumber;
  rir?: NullableNumber;
  actual_rir?: NullableNumber;
  planned_weight_kg?: NullableNumber;
  planned_min_reps?: NullableNumber;
  planned_max_reps?: NullableNumber;
  planned_rir?: NullableNumber;
  rest_seconds?: NullableNumber;
  completed_at?: string | null;
  completed?: boolean | null;
  comment?: string | null;
  notes?: string | null;
  source?: SourceName | null;
  external_id?: string | null;
};

export type ExerciseResult = {
  session_id?: number | string | null;
  date?: string | null;
  working_sets?: NullableNumber;
  repetitions?: NullableNumber;
  volume_kg?: NullableNumber;
  best_weight_kg?: NullableNumber;
  estimated_1rm?: NullableNumber;
  average_rir?: NullableNumber;
};

export type SessionExercise = {
  id?: number | string;
  exercise_id?: number | string | null;
  name?: string | null;
  exercise_name?: string | null;
  position?: number;
  note?: string | null;
  notes?: string | null;
  completed?: boolean | null;
  working_sets?: NullableNumber;
  min_reps?: NullableNumber;
  max_reps?: NullableNumber;
  target_rir?: NullableNumber;
  planned_weight_kg?: NullableNumber;
  planned_min_reps?: NullableNumber;
  planned_max_reps?: NullableNumber;
  planned_working_sets?: NullableNumber;
  planned_target_rir?: NullableNumber;
  planned_rest_seconds?: NullableNumber;
  source?: SourceName | null;
  external_id?: string | null;
  rest_after_exercise_seconds?: NullableNumber;
  current_result?: ExerciseResult | null;
  previous_result?: ExerciseResult | null;
  sets?: WorkoutSet[] | null;
};

export type WorkoutSession = {
  id: number | string;
  date?: string | null;
  actual_date?: string | null;
  scheduled_date?: string | null;
  calendar_date?: string | null;
  scheduled_at?: string | null;
  started_at?: string | null;
  finished_at?: string | null;
  duration_seconds?: NullableNumber;
  plan_id?: number | string | null;
  template_id?: number | string | null;
  plan_name?: string | null;
  template_name?: string | null;
  program_name?: string | null;
  status?: string | null;
  notes?: string | null;
  source?: SourceName | null;
  external_id?: string | null;
  has_progression_snapshot?: boolean | null;
  strain?: NullableNumber;
  working_sets?: NullableNumber;
  warmup_sets?: NullableNumber;
  total_reps?: NullableNumber;
  volume_kg?: NullableNumber;
  average_rir?: NullableNumber;
  exercises?: SessionExercise[] | null;
};

export type RecoveryEntry = {
  id: number | string;
  date: string;
  recovery_score?: NullableNumber;
  hrv_ms?: NullableNumber;
  resting_heart_rate_bpm?: NullableNumber;
  respiratory_rate?: NullableNumber;
  spo2_percent?: NullableNumber;
  skin_temperature_celsius?: NullableNumber;
  daily_strain?: NullableNumber;
  source?: SourceName | null;
  external_id?: string | null;
  notes?: string | null;
};

export type SleepEntry = {
  id: number | string;
  date?: string | null;
  sleep_start?: string | null;
  sleep_end?: string | null;
  time_in_bed_seconds?: NullableNumber;
  actual_sleep_seconds?: NullableNumber;
  awake_seconds?: NullableNumber;
  rem_seconds?: NullableNumber;
  deep_seconds?: NullableNumber;
  light_seconds?: NullableNumber;
  sleep_performance_percent?: NullableNumber;
  efficiency_percent?: NullableNumber;
  consistency_percent?: NullableNumber;
  sleep_debt_seconds?: NullableNumber;
  disturbances?: NullableNumber;
  is_nap?: boolean | null;
  source?: SourceName | null;
  external_id?: string | null;
  notes?: string | null;
};

export type NutritionDay = {
  id: number | string;
  date: string;
  calories_kcal?: NullableNumber;
  protein_g?: NullableNumber;
  fat_g?: NullableNumber;
  carbohydrates_g?: NullableNumber;
  fiber_g?: NullableNumber;
  sugar_g?: NullableNumber;
  saturated_fat_g?: NullableNumber;
  sodium_mg?: NullableNumber;
  potassium_mg?: NullableNumber;
  water_ml?: NullableNumber;
  source?: SourceName | null;
  external_id?: string | null;
  notes?: string | null;
};

export type BodySegment = {
  segment: "left_arm" | "right_arm" | "trunk" | "left_leg" | "right_leg" | string;
  lean_mass_kg?: NullableNumber;
  lean_percent?: NullableNumber;
  fat_mass_kg?: NullableNumber;
  fat_percent?: NullableNumber;
};

export type BodyMeasurement = {
  id: number | string;
  measured_at: string;
  weight_kg?: NullableNumber;
  body_fat_percent?: NullableNumber;
  fat_mass_kg?: NullableNumber;
  lean_mass_kg?: NullableNumber;
  skeletal_muscle_mass_kg?: NullableNumber;
  waist_cm?: NullableNumber;
  chest_cm?: NullableNumber;
  biceps_cm?: NullableNumber;
  thigh_cm?: NullableNumber;
  total_body_water_l?: NullableNumber;
  intracellular_water_l?: NullableNumber;
  extracellular_water_l?: NullableNumber;
  ecw_tbw_ratio?: NullableNumber;
  protein_mass_kg?: NullableNumber;
  mineral_mass_kg?: NullableNumber;
  bmi?: NullableNumber;
  visceral_fat_level?: NullableNumber;
  visceral_fat_area_cm2?: NullableNumber;
  basal_metabolic_rate_kcal?: NullableNumber;
  inbody_score?: NullableNumber;
  phase_angle_degrees?: NullableNumber;
  segments?: BodySegment[] | null;
  source?: SourceName | null;
  external_id?: string | null;
  notes?: string | null;
};

export type Exercise = {
  id: number | string;
  name: string;
  slug?: string | null;
  primary_muscle_group?: string | null;
  secondary_muscle_groups?: string[] | null;
  muscle_groups?: string[] | null;
  exercise_type?: string | null;
  equipment?: string | null;
  unilateral?: boolean | null;
  notes?: string | null;
  source?: SourceName | null;
  external_id?: string | null;
};
export type WorkoutTemplateWarmupSet = {
  position?: number | null;
  weight_kg?: NullableNumber;
  weight_mode?: "kg" | "bar" | string | null;
  bar?: boolean | null;
  reps: number;
};
export type WorkoutTemplateExercise = {
  id?: number | string;
  exercise_id?: number | string;
  name?: string;
  position: number;
  working_sets?: number | null;
  min_reps?: number | null;
  max_reps?: number | null;
  target_rir?: number | null;
  weight_step_kg?: NullableNumber;
  starting_weight_kg?: NullableNumber;
  progression_type?: "double" | string | null;
  warmup_sets?: WorkoutTemplateWarmupSet[] | null;
  rest_seconds?: number | null;
  after_seconds?: number | null;
  rest_after_exercise_seconds?: number | null;
  notes?: string | null;
};
export type WorkoutTemplate = { id?: number | string; name: string; description?: string | null; external_id?: string | null; position?: number; revision_id?: number | string | null; exercises?: WorkoutTemplateExercise[] | null };
export type WorkoutPlan = { id: number | string; name: string; description?: string | null; days_per_week?: number | null; templates?: WorkoutTemplate[] | null; historical_templates?: WorkoutTemplate[] | null; active?: boolean | null; source?: SourceName | null; external_id?: string | null };

export type Correlation = {
  id?: string;
  title: string;
  coefficient?: NullableNumber;
  sample_size?: NullableNumber;
  period?: string | null;
  definition?: string | null;
  x_label?: string | null;
  y_label?: string | null;
  points?: Array<{ x: number; y: number; date?: string }> | null;
  insufficient_sample?: boolean | null;
};

export type TrainingDurationPoint = SeriesPoint & {
  date: string;
  duration_minutes?: NullableNumber;
  average_duration_minutes?: NullableNumber;
  sessions?: number;
};

export type TrainingMuscleGroupPoint = {
  muscle_group: string;
  working_sets: number;
  volume_kg?: number;
};

export type TrainingRIRPoint = {
  rir: string;
  sets: number;
};

export type TrainingAdherencePoint = SeriesPoint & {
  date: string;
  planned: number;
  completed: number;
  adherence_percent?: NullableNumber;
};

export type TrainingHeatmapPoint = {
  date: string;
  sessions: number;
  working_sets: number;
  volume_kg?: number;
};

export type TrainingStreakSummary = {
  current_days?: number;
  longest_last_30_days?: number;
  active_days_last_30?: number;
};

export type Overview = {
  range?: { from?: string; to?: string; compare?: boolean };
  today?: Record<string, unknown> | null;
  previous_day?: Record<string, unknown> | null;
  daily?: SeriesPoint[] | null;
  summary?: Record<string, unknown> | null;
  comparison?: Record<string, unknown> | null;
  sessions?: WorkoutSession[] | null;
  today_sessions?: WorkoutSession[] | null;
  weekly_sessions?: WorkoutSession[] | null;
  weekly_range?: { from?: string; to?: string; days?: number; timezone?: string } | null;
  highlights?: Highlight[] | null;
  sources?: SourceStatus[] | null;
  nutrition?: SeriesPoint[] | null;
  body?: SeriesPoint[] | null;
  recovery?: SeriesPoint[] | null;
};

export type AnalyticsResponse = {
  metrics?: Record<string, Metric | string | number | null> | null;
  summary?: Record<string, unknown> | null;
  daily?: SeriesPoint[] | null;
  weekly?: SeriesPoint[] | null;
  series?: SeriesPoint[] | null;
  heatmap?: TrainingHeatmapPoint[] | null;
  distributions?: SeriesPoint[] | null;
  daily_duration?: TrainingDurationPoint[] | null;
  muscle_groups?: TrainingMuscleGroupPoint[] | null;
  rir_distribution?: TrainingRIRPoint[] | null;
  adherence?: TrainingAdherencePoint[] | null;
  streak?: TrainingStreakSummary | null;
  correlations?: Correlation[] | null;
  comparison?: AnalyticsResponse | null;
};

export type Settings = {
  timezone?: string | null;
  theme?: "dark" | "system" | string | null;
  first_day_of_week?: number | null;
  units?: "metric" | string | null;
  calorie_target_kcal?: NullableNumber;
  protein_target_g?: NullableNumber;
  fat_target_g?: NullableNumber;
  carbohydrates_target_g?: NullableNumber;
  sleep_target_min_seconds?: NullableNumber;
  sleep_target_max_seconds?: NullableNumber;
  recovery_ranges?: Record<string, unknown> | null;
};

export type ImportPreview = {
  import_id?: string | number;
  preview_token?: string;
  columns?: string[] | null;
  target_fields?: string[] | null;
  suggested_mapping?: Record<string, string> | null;
  required_fields?: string[] | null;
  rows?: Record<string, unknown>[] | null;
  total_rows?: number;
  valid_rows?: number;
  duplicate_rows?: number;
  invalid_rows?: number;
  errors?: Array<{ row?: number; field?: string; message: string; fields?: Record<string, string> }> | null;
};

export type ImportRun = {
  id: string | number;
  source?: string | null;
  data_type?: string | null;
  filename?: string | null;
  format?: string | null;
  status?: string | null;
  total_rows?: number | null;
  imported_rows?: number | null;
  skipped_rows?: number | null;
  failed_rows?: number | null;
  error_summary?: string | null;
  errors?: Array<{ row?: number; message: string; fields?: Record<string, string> }> | null;
  started_at?: string | null;
  completed_at?: string | null;
};
