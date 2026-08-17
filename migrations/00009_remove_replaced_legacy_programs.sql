-- +goose Up
-- Migration 00008 preserved each pre-v1 workout as a separate one-day program.
-- Once the consolidated V-shape plan has been imported, these three legacy
-- definitions only duplicate its A/B/C templates. Deleting a definition does
-- not delete completed sessions: their template/revision foreign keys use
-- ON DELETE SET NULL and their names/exercises/sets are stored as snapshots.
DELETE FROM training_programs legacy
WHERE training_normalize_exercise_name(legacy.name) IN (
          training_normalize_exercise_name('Понедельник — Фуллбади A'),
          training_normalize_exercise_name('Среда — Фуллбади B'),
          training_normalize_exercise_name('Пятница — Фуллбади C')
      )
  AND EXISTS (
      SELECT 1
      FROM training_programs replacement
      WHERE replacement.owner_id = legacy.owner_id
        AND training_normalize_exercise_name(replacement.name) =
            training_normalize_exercise_name('V-фигура · фуллбади 3 дня')
  )
  AND (
      SELECT count(*)
      FROM workout_templates template
      WHERE template.revision_id = legacy.active_revision_id
  ) = 1
  AND EXISTS (
      SELECT 1
      FROM training_program_revisions revision
      JOIN workout_templates template ON template.revision_id = revision.id
      WHERE revision.id = legacy.active_revision_id
        AND revision.program_id = legacy.id
        AND revision.raw_source = ''
        AND template.external_id = 'legacy_' || template.id
        AND training_normalize_exercise_name(template.name) =
            training_normalize_exercise_name(legacy.name)
  );

-- +goose Down
-- The removed legacy definitions cannot be reconstructed losslessly. Completed
-- sessions remain intact and the consolidated program stays active.
SELECT 1;
