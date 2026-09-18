import json
import unittest
from pathlib import Path

from invoice import summarize
from pricing import line_total


class InvoiceTests(unittest.TestCase):
    def test_quantity(self):
        self.assertEqual(line_total({"quantity": 4, "unit_price_cents": 125}), 500)

    def test_per_unit_discount(self):
        item = {"quantity": 3, "unit_price_cents": 225, "discount_cents": 25}
        self.assertEqual(line_total(item), 600)

    def test_discount_floor(self):
        item = {"quantity": 2, "unit_price_cents": 100, "discount_cents": 150}
        self.assertEqual(line_total(item), 0)

    def test_cancelled_lines(self):
        order = {"order_id": "cancelled", "items": [
            {"quantity": 2, "unit_price_cents": 150},
            {"quantity": 8, "unit_price_cents": 1000, "cancelled": True},
        ]}
        self.assertEqual(summarize(order), {
            "order_id": "cancelled", "item_count": 2, "subtotal_cents": 300,
        })

    def test_unit_count(self):
        order = {"order_id": "units", "items": [
            {"quantity": 3, "unit_price_cents": 0},
            {"quantity": 4, "unit_price_cents": 0},
        ]}
        self.assertEqual(summarize(order)["item_count"], 7)

    def test_empty_order(self):
        self.assertEqual(summarize({"order_id": "empty", "items": []}), {
            "order_id": "empty", "item_count": 0, "subtotal_cents": 0,
        })

    def test_fixture_order(self):
        order = json.loads(Path("order.json").read_text())
        expected = json.loads(Path("expected.json").read_text())
        self.assertEqual(summarize(order), expected)


if __name__ == "__main__":
    unittest.main()
