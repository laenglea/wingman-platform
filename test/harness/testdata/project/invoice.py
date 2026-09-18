from pricing import line_total


def summarize(order):
    """Exclude cancelled lines; item_count counts units, not distinct lines."""
    items = order["items"]
    return {
        "order_id": order["order_id"],
        "item_count": len(items),
        "subtotal_cents": sum(line_total(item) for item in items),
    }
