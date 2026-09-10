function dimension(raw) {
  if (typeof raw !== "string" || !/^[0-9]{1,4}$/.test(raw)) return 0;
  const value = Number(raw);
  return value >= 1 && value <= 2000 ? value : 0;
}

function main(input) {
  if (input.query.width_cm === undefined && input.query.length_cm === undefined) {
    return { valid: false, widthInput: "400", lengthInput: "500", message: "" };
  }
  const widthInput = typeof input.query.width_cm === "string" ? input.query.width_cm : "";
  const lengthInput = typeof input.query.length_cm === "string" ? input.query.length_cm : "";
  const width = dimension(input.query.width_cm);
  const length = dimension(input.query.length_cm);
  if (!width || !length) {
    return { valid: false, widthInput: widthInput, lengthInput: lengthInput,
      message: "Geef breedte en lengte als gehele centimeters van 1 tot 2000, elk eenmaal." };
  }

  const boxArea = Number(input.values.area_per_box_cm2);
  const waste = Number(input.values.waste_percent);
  if (!Number.isInteger(boxArea) || boxArea < 1 || boxArea > 1000000 ||
      !Number.isInteger(waste) || waste < 0 || waste > 100) {
    throw new Error("Ongeldige materiaalconfiguratie.");
  }

  // Calculate in square centimeters; format square meters only for display.
  const floorArea = width * length;
  const requiredArea = Math.ceil(floorArea * (100 + waste) / 100);
  const boxes = Math.ceil(requiredArea / boxArea);
  return {
    valid: true,
    widthInput: widthInput,
    lengthInput: lengthInput,
    widthCm: width,
    lengthCm: length,
    floorM2: (floorArea / 10000).toFixed(2),
    requiredM2: (requiredArea / 10000).toFixed(2),
    boxes: boxes,
    suppliedM2: (boxes * boxArea / 10000).toFixed(2)
  };
}
