package harness

// ReasoningPrompt is deliberately harder than counting letters or apples.
// Adaptive models legitimately answer those trivial fixtures without thinking,
// which cannot exercise signed reasoning replay or reasoning-token accounting.
const ReasoningPrompt = "Find the smallest positive integer x such that x mod 7 = 3, x mod 11 = 5, x mod 13 = 7, and x mod 17 = 11. Solve step by step and verify every remainder."
