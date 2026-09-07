export const unitPrices = { boards: 3250, supports: 1800, fasteners: 2400 } as const;

export function calculate(quantities: Record<keyof typeof unitPrices, number>, includeTax: boolean) {
  const subtotal = Object.entries(unitPrices).reduce((sum, [item, cents]) =>
    sum + cents * quantities[item as keyof typeof unitPrices], 0);
  const tax = includeTax ? Math.round(subtotal * 21 / 100) : 0;
  return { subtotal, tax, total: subtotal + tax };
}
