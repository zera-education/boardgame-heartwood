package main

// Card text, treasures and roles. The six values on the map are ZERA's core
// values, ZERAOS. Each ring has its own deck of questions, from light to deep.

// Value is one of ZERA's core values. The map's wedges follow them clockwise,
// so the forest spells Z-E-R-A-O-S. Each player stands for one.
type Value struct {
	Letter      string `json:"letter"`
	Name        string `json:"name"`
	Tagline     string `json:"tagline"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
}

var Values = [6]Value{
	{"Z", "Zealous", "Be zealous, not jealous.", "Passionate, driven energy channelled into learning and purpose rather than comparison: advocacy for students, initiative, and enthusiastic participation.", "☀️", "#e0703a"},
	{"E", "Excellence", "Beyond expectation.", "Consistently surpassing standards, never merely meeting them: in academics, governance, and personal growth. Heart (zealous) always comes first.", "⭐", "#e0a32e"},
	{"R", "Resilience", "Anti-fragile.", "More than bouncing back: growing stronger through challenge. Purpose-driven leadership that persists toward long-term goals and learns from setbacks.", "🎋", "#c8463f"},
	{"A", "Authenticity", "Inclusive education.", "Genuine representation and honesty. Everyone, including students in zera Plus, belongs and is valued, without pretense.", "🪞", "#cf6f97"},
	{"O", "Open-mindedness", "Growth mindset.", "Welcoming diverse perspectives and inclusive decision-making: challenging assumptions and treating failure as a place to grow, not an endpoint.", "💡", "#4f74b0"},
	{"S", "Sustainability", "Start with the end in mind.", "Long-term thinking built into everything: creating lasting impact and building systems that outlast any single leader or cohort.", "🌍", "#3f9e9a"},
}

// RingDecks holds the card a player draws the first time they reach a ring
// (index 1..3; 0 is unused). Questions to get to know each other, from light
// at the edge to deep near the World Tree: Ring 3 the Light deck, Ring 2 the
// Story deck, Ring 1 the Deep deck. Twelve each, one per player in the
// biggest game, so nobody hears the same card twice.
var RingDecks = [4][]string{
	1: {
		"What makes you feel most like yourself?",
		"What are you most grateful for right now?",
		"Who has shaped the person you are today?",
		"What is something you're still learning about yourself?",
		"What keeps you going when work gets hard?",
		"What has life taught you that school never did?",
		"What is one thing you will never compromise on?",
		"What would you try if you knew you couldn't fail?",
		"What is a dream you still carry?",
		"When do you feel you truly belong?",
		"What would you tell your younger self?",
		"What kind of support helps you most when you're struggling?",
	},
	2: {
		"Tell us about a teacher who made a difference to you.",
		"Tell us about the place where you grew up.",
		"Tell us how you ended up working at ZERA.",
		"Tell us about a time you tried something new and it went well.",
		"Tell us about a mistake that taught you a lot.",
		"Tell us about someone who helped you when you needed it.",
		"Tell us about a moment you felt really proud of yourself.",
		"Tell us about a time a plan went wrong, and what happened next.",
		"Tell us about a tradition in your family.",
		"Tell us about a student who taught you something. (No names needed.)",
		"Tell us about your first week in a new job.",
		"Tell us about a challenge that made you stronger.",
	},
	3: {
		"What's your favourite food from your childhood?",
		"What was your first job?",
		"What's a small thing that always makes your day better?",
		"Where is your favourite place to relax?",
		"What did you want to be when you were a child?",
		"What's a hobby most of us don't know you have?",
		"What's the best trip you've ever taken?",
		"What song always puts you in a good mood?",
		"Are you a morning person or a night person?",
		"What's something you're surprisingly good at?",
		"What's the last thing that made you laugh out loud?",
		"If you had a free day tomorrow, how would you spend it?",
	},
}

// RingDeckNames names each ring's deck, by ring (0 is unused).
var RingDeckNames = [4]string{"", "Deep", "Story", "Light"}

// HeartwoodCards: one is asked each time a player places fruit on the World Tree.
var HeartwoodCards = []string{
	"What do you want the people at this table to know about you that they might not?",
	"Who at this table has helped you grow, and how?",
	"What legacy do you want to leave in ZERA's forest?",
	"What would you want a student to say about you in 2035?",
	"What have you noticed about this team today?",
	"What promise would you make to this team for the year ahead?",
	"What has this team given you that you didn't expect?",
	"Which value do you want this team to grow in most this year, and why?",
	"What will you do differently after today?",
	"Who here would you like to know better, and what would you ask them?",
	"What makes you proud to work at ZERA?",
	"Who at this table do you want to thank, and for what?",
	"What would a 'woken forest' look like at our school?",
	"When did you last see this team at its best?",
}

// HarvestPrompt is the share after a Harvest; {value} is the tree's value.
// The value's tagline is shown under it.
const HarvestPrompt = "Tell us a story from your own life about {value}."

// Treasure is one of the three named treasures hidden in Rings 1 and 2. When
// found, the whole table answers its question in one sentence each.
type Treasure struct {
	ID       string `json:"id"`
	Icon     string `json:"icon"`
	Name     string `json:"name"`
	Meaning  string `json:"meaning"`
	Question string `json:"question"`
}

var Treasures = []Treasure{
	{"compass", "🧭", "The Compass", "Purpose: where we're going.", "Everyone, one sentence: where do you want this team to be a year from now?"},
	{"lantern", "🏮", "The Lantern", "Lighting the way for others.", "Everyone, one sentence: who lit the way for you when you were new?"},
	{"rope", "🪢", "The Rope", "Holding together.", "Everyone, one sentence: what holds this team together when things get hard?"},
}

// HeartComplete is said when all three treasures are on the World Tree.
const HeartComplete = "Purpose, light and each other: the heart of the forest is complete."

func treasureByID(id string) *Treasure {
	for i := range Treasures {
		if Treasures[i].ID == id {
			return &Treasures[i]
		}
	}
	return nil
}

// Role is one Enneagram type's gift in the forest. The Keeper gives each player
// 1 to 3 types in the lobby; they are always on.
type Role struct {
	Type int    `json:"type"`
	Name string `json:"name"`
	Gift string `json:"gift"`
	Text string `json:"text"`
}

var Roles = []Role{
	{1, "Reformer", "integrity", "When you Clear, remove 2 layers (still 1 action in rain)."},
	{2, "Helper", "care", "Water, Tend or Clear a hex next to you without standing on it."},
	{3, "Achiever", "drive", "After you Sow, the hex grows straight to Sprout."},
	{4, "Individualist", "depth", "When you Explore, also peek at one unexplored hex next to you."},
	{5, "Investigator", "insight", "See which hexes the next Forest Breath will hit; once a round, see one sector's next weather."},
	{6, "Loyalist", "loyalty", "Plants on your hex and the 6 hexes around you (7 in all) are safe from dead leaves."},
	{7, "Enthusiast", "joy", "One Move a turn may be 2 hexes (not in fog, not through sealed hexes)."},
	{8, "Challenger", "strength", "You may enter sealed hexes, and bring one teammate from your hex when you Move."},
	{9, "Peacemaker", "harmony", "You and a teammate on a hex next to yours can pass things to each other."},
}

var Colors = []string{"#e53935", "#fb8c00", "#fdd835", "#43a047", "#00897b", "#1e88e5", "#5e35b1", "#d81b60", "#6d4c41", "#546e7a", "#00acc1", "#7cb342"}
