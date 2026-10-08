package main

// Card text, treasures and roles. The six values on the map are ZERA's core
// values, ZERAOS; each value has five cards with a Light, a Story and a Deep
// prompt, pooled into one deck per ring.

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

// valueCards is each value's five cards, each with a Light, a Story and a Deep
// prompt. The ring decks are built from it.
var valueCards = [6][][3]string{
	{ // Zealous
		{"What could you do for hours without noticing the time?", "Tell us about a time you got other people excited about something you love.", "What makes you care so much about your work?"},                                   // What lights you up
		{"Whose good news made you really happy this year?", "Tell us about a time someone else's success made you want to try harder.", "If you stopped comparing yourself with others, what would you do differently?"},           // Cheering, not comparing
		{"When someone asks for a volunteer, are you the first to raise your hand, or the last?", "Tell us about a time you helped before anyone asked you to.", "What have you been wanting to start, but haven't yet?"},           // Stepping up first
		{"What's the best thing a child or student has ever said to you?", "Tell us about a time you did something extra for a young person who needed it.", "Which students do you think are easiest to miss? (No names needed.)"}, // In young people's corner
		{"What will you always say yes to, even when you're tired?", "Tell us about the last time you tried something new. How did it go?", "What makes you hesitate to join in?"},                                                  // Joining in wholeheartedly
	},
	{ // Excellence
		{"What's the best service you've ever had in a shop, café or restaurant?", "Tell us about someone who did far more than you expected of them.", "What would you like to do much better this year?"},                              // Beyond what was asked
		{"What do you do that nobody notices when it's done well?", "Tell us about a time you did something properly even though no one would check. Why did you?", "What rule do you keep for yourself that nobody asked you to keep?"}, // Doing it right when no one's looking
		{"Who was the strictest but kindest teacher or boss you've had?", "Tell us about a time you chose to be kind instead of getting it perfect.", "How do you push for high standards and still stay kind?"},                         // Heart comes first
		{"What's the best team you've ever been part of?", "Tell us about a time your team did better than anyone expected.", "What do you wish your team could do really well together?"},                                               // Raising the bar together
		{"Name something you're much better at now than five years ago.", "Tell us how you got good at something that once felt impossible.", "How do you know when something is good enough?"},                                          // Getting better, kindly
	},
	{ // Resilience
		{"What's the funniest story behind a scar, or something of yours that's broken?", "Tell us about a hard time, big or small, that made you stronger.", "What did a hard time teach you that you're now glad to know?"},        // Stronger because of it
		{"What's the best thing that happened to you because a plan went wrong?", "Tell us about a time something went wrong and you found a better way.", "When your plans go wrong, how do you usually react?"},                    // When plans go wrong
		{"What advice did you ignore at first, but later realised was right?", "Tell us about some hard feedback that ended up helping you.", "What kind of feedback is still hard for you to hear?"},                                // Hard words that helped
		{"What's your secret trick for keeping calm when everything happens at once?", "Tell us about a time you stayed calm because others needed you to.", "What helps you stay strong for others when you're struggling inside?"}, // Steady for others
		{"What always cheers you up on a bad day?", "Tell us about someone who helped you keep going when you wanted to give up.", "Is it easy or hard for you to ask for help? Why?"},                                               // Not going it alone
	},
	{ // Authenticity
		{"Where do you feel most relaxed?", "Tell us about a time you felt left out, and what helped you feel welcome.", "What makes you feel you truly belong somewhere?"},                                                                                               // Belonging, not just fitting in
		{"What did you once pretend to like, just to fit in?", "Tell us about a time you were honest about who you are, and it went better than you expected.", "When do you feel most like yourself?"},                                                                   // Dropping the act
		{"What does everyone assume you can do, but you can't?", "Tell us about a time saying “I don't know” helped.", "When do you feel you have to pretend you know all the answers?"},                                                                                  // Not having all the answers
		{"What's a word or food from home that you always have to explain?", "Tell us about a time sharing where you come from brought you closer to someone.", "Which part of where you come from do you most want to bring into your work here?"},                       // Where you come from
		{"How do you learn a new gadget: read the manual, watch a video, or just try?", "Tell us about someone who explained something hard in a way you finally understood.", "What have you learned from someone who sees or learns things very differently from you?"}, // Many ways to learn
	},
	{ // Open-mindedness
		{"What did you believe as a child that turned out to be completely untrue?", "Tell us about a time someone changed your mind about something that mattered.", "What's something you find very hard to change your mind about?"},                // Changing your mind
		{"What's something you're bad at, and don't mind?", "Tell us about a mistake that taught you a lot.", "What has failure taught you that success never could?"},                                                                                 // Learning from mistakes
		{"Name a food you were sure you'd hate, until you tried it.", "Tell us about a time your first impression of someone was completely wrong. (No names needed.)", "When are you most likely to judge too quickly?"},                              // First impressions
		{"What silly debate could you argue about forever?", "Tell us about a time a disagreement led to a better decision.", "How do you react when someone disagrees with you?"},                                                                     // Disagreeing well
		{"What's one thing from another culture that's now part of your everyday life?", "Tell us about someone very different from you who taught you something you still use.", "How do you feel when someone sees life very differently from you?"}, // Learning across differences
	},
	{ // Sustainability
		{"What small habit have you kept for years?", "Tell us about a habit that changed your life.", "What habit are you trying to build right now?"},                                                                                                     // Habits that last
		{"What simple trick makes your daily life easier?", "Tell us about something you set up that still worked well when you weren't there.", "What part of your work would be hardest to hand over to someone else?"},                                   // Things that run without you
		{"If you could plant one tree anywhere in the world, where would it go?", "Tell us about something someone started long ago that you're grateful for today.", "What would you gladly start, even if someone else got the credit for finishing it?"}, // Planting for others
		{"What's the longest you've ever waited for something, and was it worth it?", "Tell us about something you worked on for years before you saw the results.", "When are you tempted to take a shortcut?"},                                            // The long game
		{"What would you put in a time capsule to open in 2050?", "Tell us about a big goal you planned step by step. How did it go?", "Ten years from now, what do you hope you'll thank yourself for doing this year?"},                                   // Starting with the end in mind
	},
}

// RegionThreads names each value card's theme (shown in the wiki): ringDecks[r][v*5+k]
// comes from value v's card k, whose theme is RegionThreads[v][k].
var RegionThreads = [6][]string{
	{"What lights you up", "Cheering, not comparing", "Stepping up first", "In young people's corner", "Joining in wholeheartedly"},
	{"Beyond what was asked", "Doing it right when no one's looking", "Heart comes first", "Raising the bar together", "Getting better, kindly"},
	{"Stronger because of it", "When plans go wrong", "Hard words that helped", "Steady for others", "Not going it alone"},
	{"Belonging, not just fitting in", "Dropping the act", "Not having all the answers", "Where you come from", "Many ways to learn"},
	{"Changing your mind", "Learning from mistakes", "First impressions", "Disagreeing well", "Learning across differences"},
	{"Habits that last", "Things that run without you", "Planting for others", "The long game", "Starting with the end in mind"},
}

// RingDecks holds the card a player draws the first time they reach a ring
// (index 1..3; 0 is unused): Ring 3 the Light deck, Ring 2 the Story deck,
// Ring 1 the Deep deck, pooled across the six values (30 each).
var RingDecks = buildRingDecks()

// RingDeckNames names each ring's deck, by ring (0 is unused).
var RingDeckNames = [4]string{"", "Deep", "Story", "Light"}

func buildRingDecks() [4][]string {
	var d [4][]string
	for _, cards := range valueCards {
		for _, c := range cards {
			d[3] = append(d[3], c[0])
			d[2] = append(d[2], c[1])
			d[1] = append(d[1], c[2])
		}
	}
	return d
}

// HeartwoodCards: one is asked each time a player places fruit on the World Tree.
var HeartwoodCards = []string{
	"What do you want the people at this table to know about you that they might not?",
	"Who at this table has helped you grow, and how?",
	"What legacy do you want to leave in ZERA's forest?",
	"What would you want a student to say about you in 2035?",
	"What did today show you about this team?",
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
	Text string `json:"text"`
}

var Roles = []Role{
	{1, "Reformer", "When you Clear, remove 2 layers (still 1 action in rain)."},
	{2, "Helper", "Water, Tend or Clear a hex next to you without standing on it."},
	{3, "Achiever", "After you Sow, the hex grows straight to Sprout."},
	{4, "Individualist", "When you Explore, also peek at one unexplored hex next to you."},
	{5, "Investigator", "See which hexes the next Forest Breath will hit; once a round, see one sector's next weather."},
	{6, "Loyalist", "Plants on your hex and the 6 hexes around you (7 in all) are safe from dead leaves."},
	{7, "Enthusiast", "One Move a turn may be 2 hexes (not in fog, not through sealed hexes)."},
	{8, "Challenger", "You may enter sealed hexes, and bring one teammate from your hex when you Move."},
	{9, "Peacemaker", "You and a teammate on a hex next to yours can pass things to each other."},
}

var Colors = []string{"#e53935", "#fb8c00", "#fdd835", "#43a047", "#00897b", "#1e88e5", "#5e35b1", "#d81b60", "#6d4c41", "#546e7a", "#00acc1", "#7cb342"}
