import { describe, expect, it } from "vitest";
import { adaptersForDataType, resolveImportAdapter } from "./import-adapters";

describe("import adapters", () => {
  it("marks gym InBody files with the inbody source", () => {
    expect(resolveImportAdapter("inbody_csv")).toMatchObject({ format: "csv", source: "inbody" });
    expect(resolveImportAdapter("inbody_json")).toMatchObject({ format: "json", source: "inbody" });
  });

  it("offers InBody adapters only for body measurements", () => {
    expect(adaptersForDataType("body").map((adapter) => adapter.value)).toContain("inbody_csv");
    expect(adaptersForDataType("workouts").map((adapter) => adapter.value)).not.toContain("inbody_csv");
  });
});
