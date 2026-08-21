-- +goose Up
ALTER TABLE body_measurements
    DROP CONSTRAINT IF EXISTS body_measurements_check;

ALTER TABLE body_measurements
    ADD COLUMN total_body_water_l            NUMERIC(8, 3) CHECK (total_body_water_l > 0),
    ADD COLUMN intracellular_water_l         NUMERIC(8, 3) CHECK (intracellular_water_l > 0),
    ADD COLUMN extracellular_water_l         NUMERIC(8, 3) CHECK (extracellular_water_l > 0),
    ADD COLUMN ecw_tbw_ratio                 NUMERIC(6, 5) CHECK (ecw_tbw_ratio > 0 AND ecw_tbw_ratio <= 1),
    ADD COLUMN protein_mass_kg               NUMERIC(8, 3) CHECK (protein_mass_kg >= 0),
    ADD COLUMN mineral_mass_kg               NUMERIC(8, 3) CHECK (mineral_mass_kg >= 0),
    ADD COLUMN bmi                           NUMERIC(6, 2) CHECK (bmi > 0),
    ADD COLUMN visceral_fat_level            SMALLINT CHECK (visceral_fat_level >= 0),
    ADD COLUMN visceral_fat_area_cm2         NUMERIC(9, 2) CHECK (visceral_fat_area_cm2 >= 0),
    ADD COLUMN basal_metabolic_rate_kcal     NUMERIC(9, 2) CHECK (basal_metabolic_rate_kcal > 0),
    ADD COLUMN inbody_score                  NUMERIC(6, 2) CHECK (inbody_score >= 0),
    ADD COLUMN phase_angle_degrees           NUMERIC(5, 2) CHECK (phase_angle_degrees > 0 AND phase_angle_degrees < 90),
    ADD CONSTRAINT body_measurements_has_measurement_check CHECK (
        weight_kg IS NOT NULL OR body_fat_percent IS NOT NULL OR fat_mass_kg IS NOT NULL
        OR lean_mass_kg IS NOT NULL OR skeletal_muscle_mass_kg IS NOT NULL OR waist_cm IS NOT NULL
        OR chest_cm IS NOT NULL OR biceps_cm IS NOT NULL OR thigh_cm IS NOT NULL
        OR total_body_water_l IS NOT NULL OR intracellular_water_l IS NOT NULL
        OR extracellular_water_l IS NOT NULL OR ecw_tbw_ratio IS NOT NULL
        OR protein_mass_kg IS NOT NULL OR mineral_mass_kg IS NOT NULL OR bmi IS NOT NULL
        OR visceral_fat_level IS NOT NULL OR visceral_fat_area_cm2 IS NOT NULL
        OR basal_metabolic_rate_kcal IS NOT NULL OR inbody_score IS NOT NULL
        OR phase_angle_degrees IS NOT NULL
    ),
    ADD CONSTRAINT body_measurements_id_owner_key UNIQUE (id, owner_id);

CREATE TABLE body_segment_measurements (
    id                  BIGSERIAL PRIMARY KEY,
    body_measurement_id BIGINT NOT NULL,
    owner_id            BIGINT NOT NULL,
    segment             TEXT NOT NULL CHECK (segment IN ('left_arm', 'right_arm', 'trunk', 'left_leg', 'right_leg')),
    lean_mass_kg        NUMERIC(8, 3) CHECK (lean_mass_kg >= 0),
    lean_percent        NUMERIC(7, 2) CHECK (lean_percent >= 0),
    fat_mass_kg         NUMERIC(8, 3) CHECK (fat_mass_kg >= 0),
    fat_percent         NUMERIC(7, 2) CHECK (fat_percent >= 0),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT body_segment_measurements_parent_fk
        FOREIGN KEY (body_measurement_id, owner_id)
        REFERENCES body_measurements (id, owner_id)
        ON DELETE CASCADE,
    CONSTRAINT body_segment_measurements_measurement_segment_key
        UNIQUE (body_measurement_id, segment),
    CONSTRAINT body_segment_measurements_has_measurement_check CHECK (
        lean_mass_kg IS NOT NULL OR lean_percent IS NOT NULL
        OR fat_mass_kg IS NOT NULL OR fat_percent IS NOT NULL
    )
);

CREATE INDEX body_segment_measurements_owner_idx
    ON body_segment_measurements (owner_id, body_measurement_id);

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM body_segment_measurements)
        OR EXISTS (
            SELECT 1 FROM body_measurements
            WHERE total_body_water_l IS NOT NULL OR intracellular_water_l IS NOT NULL
                OR extracellular_water_l IS NOT NULL OR ecw_tbw_ratio IS NOT NULL
                OR protein_mass_kg IS NOT NULL OR mineral_mass_kg IS NOT NULL OR bmi IS NOT NULL
                OR visceral_fat_level IS NOT NULL OR visceral_fat_area_cm2 IS NOT NULL
                OR basal_metabolic_rate_kcal IS NOT NULL OR inbody_score IS NOT NULL
                OR phase_angle_degrees IS NOT NULL
        ) THEN
        RAISE EXCEPTION 'refusing to drop InBody composition data';
    END IF;
END $$;
-- +goose StatementEnd

DROP TABLE body_segment_measurements;

ALTER TABLE body_measurements
    DROP CONSTRAINT body_measurements_id_owner_key,
    DROP CONSTRAINT body_measurements_has_measurement_check,
    DROP COLUMN total_body_water_l,
    DROP COLUMN intracellular_water_l,
    DROP COLUMN extracellular_water_l,
    DROP COLUMN ecw_tbw_ratio,
    DROP COLUMN protein_mass_kg,
    DROP COLUMN mineral_mass_kg,
    DROP COLUMN bmi,
    DROP COLUMN visceral_fat_level,
    DROP COLUMN visceral_fat_area_cm2,
    DROP COLUMN basal_metabolic_rate_kcal,
    DROP COLUMN inbody_score,
    DROP COLUMN phase_angle_degrees,
    ADD CONSTRAINT body_measurements_check CHECK (
        weight_kg IS NOT NULL OR body_fat_percent IS NOT NULL OR fat_mass_kg IS NOT NULL
        OR lean_mass_kg IS NOT NULL OR skeletal_muscle_mass_kg IS NOT NULL OR waist_cm IS NOT NULL
        OR chest_cm IS NOT NULL OR biceps_cm IS NOT NULL OR thigh_cm IS NOT NULL
    );
