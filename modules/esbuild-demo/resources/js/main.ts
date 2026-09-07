import { calculate, unitPrices } from "./calculate";

const form = document.querySelector<HTMLFormElement>("#estimate-form")!;
const money = new Intl.NumberFormat("en-IE", { style: "currency", currency: "EUR" });
const field = (id: string) => document.getElementById(id)!;
const quantity = (id: string) => (form.elements.namedItem(id) as HTMLInputElement).valueAsNumber;

function update() {
  const valid = form.checkValidity();
  field("validation").textContent = valid ? "" : "Enter whole quantities between 0 and 10,000.";
  if (!valid) {
    for (const id of ["subtotal", "tax-total", "grand-total"]) field(id).textContent = "--";
    return;
  }
  const includeTax = (form.elements.namedItem("tax") as HTMLInputElement).checked;
  const quantities = Object.fromEntries(Object.keys(unitPrices).map(item => [item, quantity(item)])) as Record<keyof typeof unitPrices, number>;
  const result = calculate(quantities, includeTax);
  field("subtotal").textContent = money.format(result.subtotal / 100);
  field("tax-total").textContent = money.format(result.tax / 100);
  field("grand-total").textContent = money.format(result.total / 100);
  field("tax-rate").textContent = includeTax ? "21%" : "0%";
}

form.addEventListener("input", update);
form.addEventListener("submit", event => event.preventDefault());
form.addEventListener("reset", () => requestAnimationFrame(update));
update();
