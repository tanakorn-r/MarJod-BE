package model

var defaultQuestPresets = []QuestPreset{
	{Key: "daily_log_transaction", Name: "Log a transaction today", Logo: "✍️", Period: QuestPeriodDaily, Difficulty: QuestBasic, XP: 10, Accent: "#3D9B6E", RuleType: QuestRuleLogTransactionCount, Target: 1, Unit: "count", IsActive: true},
	{Key: "daily_under_allowance_1800", Name: "Stay under ฿1,800 today", Logo: "🛡️", Period: QuestPeriodDaily, Difficulty: QuestAdvanced, XP: 15, Accent: "#E6A23C", RuleType: QuestRuleSpendCap, Target: 1800, Unit: "thb", IsActive: true},
	{Key: "daily_no_impulse", Name: "Avoid impulse buys today", Logo: "🧘", Period: QuestPeriodDaily, Difficulty: QuestAdvanced, XP: 10, Accent: "#B5829B", RuleType: QuestRuleNoImpulse, Target: 0, Unit: "count", BehaviorTag: "impulse", IsActive: true},
	{Key: "no_spend_2", Name: "Two no-spend days", Logo: "🌿", Period: QuestPeriodWeekly, Difficulty: QuestBasic, XP: 60, Accent: "#3D9B6E", RuleType: QuestRuleNoSpendDays, Target: 2, Unit: "days", IsActive: true},
	{Key: "coffee_cap_basic_1000", Name: "Basic coffee quest: spend under ฿1000 this week", Logo: "☕", Period: QuestPeriodWeekly, Difficulty: QuestBasic, XP: 30, Accent: "#D9463B", RuleType: QuestRuleCoffeeSpendCap, Target: 1000, Unit: "thb", IsActive: true},
	{Key: "coffee_cap_advanced_500", Name: "Advanced coffee quest: spend under ฿500 this week", Logo: "☕", Period: QuestPeriodWeekly, Difficulty: QuestAdvanced, XP: 50, Accent: "#E0714E", RuleType: QuestRuleCoffeeSpendCap, Target: 500, Unit: "thb", IsActive: true},
	{Key: "coffee_cap_expert_300", Name: "Expert coffee quest: spend under ฿300 this week", Logo: "☕", Period: QuestPeriodWeekly, Difficulty: QuestExpert, XP: 75, Accent: "#E6A23C", RuleType: QuestRuleCoffeeSpendCap, Target: 300, Unit: "thb", IsActive: true},
	{Key: "coffee_cap_master_200", Name: "Master coffee quest: spend under ฿200 this week", Logo: "☕", Period: QuestPeriodWeekly, Difficulty: QuestMaster, XP: 110, Accent: "#B5829B", RuleType: QuestRuleCoffeeSpendCap, Target: 200, Unit: "thb", IsActive: true},
	{Key: "coffee_cap_grand_master_100", Name: "Grand Master coffee quest: spend under ฿100 this week", Logo: "☕", Period: QuestPeriodWeekly, Difficulty: QuestGrandMaster, XP: 160, Accent: "#241F1A", RuleType: QuestRuleCoffeeSpendCap, Target: 100, Unit: "thb", IsActive: true},
	{Key: "beat_last_month_savings", Name: "Beat last month's save rate", Logo: "📈", Period: QuestPeriodWeekly, Difficulty: QuestExpert, XP: 100, Accent: "#E6A23C", RuleType: QuestRuleBeatLastMonthSavingsRate, Unit: "percent", IsActive: true},
	{Key: "no_impulse_week", Name: "Zero impulse buys this week", Logo: "🧠", Period: QuestPeriodWeekly, Difficulty: QuestAdvanced, XP: 50, Accent: "#B5829B", RuleType: QuestRuleNoImpulse, Target: 0, Unit: "count", BehaviorTag: "impulse", IsActive: true},
	{Key: "log_5", Name: "Log 5 transactions this week", Logo: "🧾", Period: QuestPeriodWeekly, Difficulty: QuestBasic, XP: 30, Accent: "#8C9CB0", RuleType: QuestRuleLogTransactionCount, Target: 5, Unit: "count", IsActive: true},
}

func DefaultQuestPresets() []QuestPreset {
	presets := make([]QuestPreset, len(defaultQuestPresets))
	copy(presets, defaultQuestPresets)
	return presets
}
