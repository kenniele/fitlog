export type ImportAdapter = {
  value: string;
  label: string;
  format: "csv" | "json";
  source: "manual" | "whoop" | "fatsecret" | "inbody";
  dataType?: string;
};

export const importAdapters: ImportAdapter[] = [
  { value: "csv", label: "Обычный CSV", format: "csv", source: "manual" },
  { value: "json", label: "Обычный JSON", format: "json", source: "manual" },
  { value: "whoop_csv", label: "WHOOP export (файл)", format: "csv", source: "whoop" },
  { value: "fatsecret_csv", label: "FatSecret export (файл)", format: "csv", source: "fatsecret" },
  { value: "inbody_csv", label: "InBody CSV", format: "csv", source: "inbody", dataType: "body" },
  { value: "inbody_json", label: "InBody JSON", format: "json", source: "inbody", dataType: "body" },
];

export function adaptersForDataType(dataType: string) {
  return importAdapters.filter((adapter) => adapter.dataType == null || adapter.dataType === dataType);
}

export function resolveImportAdapter(value: string) {
  return importAdapters.find((adapter) => adapter.value === value) ?? importAdapters[0];
}
