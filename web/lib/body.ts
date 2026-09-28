export function relativeDifference(
  left?: number | null,
  right?: number | null,
) {
  if (
    typeof left !== "number" ||
    typeof right !== "number" ||
    !Number.isFinite(left) ||
    !Number.isFinite(right)
  )
    return null;
  const average = (Math.abs(left) + Math.abs(right)) / 2;
  return average === 0 ? null : (Math.abs(left - right) / average) * 100;
}
