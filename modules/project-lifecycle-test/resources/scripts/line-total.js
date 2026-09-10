// Trusted, synchronous project code. Each render gets fresh input and state.
function main(input) {
  const priceCents = Number(input.values.unit_price_cents);
  const maxQuantity = Number(input.values.max_quantity);
  const rawQuantity = input.query.quantity;
  const quantity = rawQuantity === undefined ? 1 : Number(rawQuantity);

  // Repeated ?quantity= values arrive as an array; do not silently choose one.
  const valid = !Array.isArray(rawQuantity)
    && Number.isInteger(quantity)
    && quantity >= 1
    && quantity <= maxQuantity;

  return {
    valid,
    quantity: valid ? quantity : "",
    max_quantity: maxQuantity,
    unit_price: (priceCents / 100).toFixed(2),
    total: valid ? (quantity * priceCents / 100).toFixed(2) : "",
    message: valid
      ? `${quantity} ${quantity === 1 ? "item" : "items"} at €${(priceCents / 100).toFixed(2)} each.`
      : `Choose one whole quantity from 1 to ${maxQuantity}.`
  };
}
