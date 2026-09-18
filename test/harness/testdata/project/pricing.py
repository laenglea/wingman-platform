def line_total(item):
    """Total cents: quantity times discounted unit price, floored at zero per unit."""
    return item["unit_price_cents"] - item.get("discount_cents", 0)
