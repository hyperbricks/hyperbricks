function main(input) {
  const quantity = Number(input.query.quantity || 1);
  const stock = Number(input.values.stock);

  if (!Number.isInteger(quantity) || quantity < 1 || quantity > 10000) {
    return { message: "Kies een geheel aantal tussen 1 en 10000." };
  }

  return {
    message: quantity <= stock ? "Op voorraad" : "Onvoldoende voorraad"
  };
}