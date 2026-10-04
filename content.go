package main

// Card text and powers. Regions follow ZERA's six Cs; each region card has a
// Seed (light), Sapling (story) and Oak (deep) prompt.

var RegionNames = [6]string{"Curiosity", "Creativity", "Collaboration", "Courage", "Compassion", "Commitment"}

var RegionCards = [6][][3]string{
	{ // Curiosity
		{"What could you talk about for ten minutes with no notes?", "Tell us about a question that changed the direction of your life.", "What about yourself are you still trying to figure out?"},
		{"If you could master one skill overnight, what would it be?", "Who first made you love learning, and how?", "What do you wish people here asked you about more?"},
		{"What's the last thing you looked up out of pure curiosity?", "Tell us about a place that opened your eyes.", "Which belief of yours has changed the most over the years?"},
		{"Which book, film or show do you recommend to everyone?", "Tell us about someone very different from you who taught you something.", "What question are you carrying into next year?"},
		{"What job would you try for one week, just to see?", "Tell us about a mistake that taught you more than any success.", "What would you explore if no one depended on you?"},
	},
	{ // Creativity
		{"What did you love making as a child?", "Tell us about a time you solved a problem in a way nobody expected.", "What part of you rarely gets to show up at work?"},
		{"What's your favourite way to spend a free Saturday?", "Tell us about something you started from nothing.", "If you knew you couldn't fail, what would you build next?"},
		{"What creative skill do you secretly wish you had?", "Tell us about a time you turned a constraint into an advantage.", "When do you feel most fully yourself?"},
		{"What would your perfect classroom look like?", "Tell us about an idea of yours that people doubted.", "What dream did you put down that you'd like to pick up again?"},
		{"Describe your ideal weekend in three words.", "Tell us about a moment of play you still remember.", "What does the world lose if you play it safe?"},
	},
	{ // Collaboration
		{"What's the best team you've ever been part of (sport, choir, class project, anything)?", "Tell us about someone who made you better just by working beside you.", "What do you need from this team that you've never asked for?"},
		{"Planner or \"let's figure it out\"? Give an example.", "Tell us about a disagreement that ended up making a relationship stronger.", "When do you feel most alone in your role?"},
		{"In a group, are you the starter, the finisher, the connector or the critic?", "Tell us about a time a team carried you.", "What makes it hard for you to ask for help?"},
		{"Who's the person you call when something goes wrong?", "Tell us about a time you had to rebuild trust with someone.", "What do you want this team to understand about how you work?"},
		{"What's a team ritual you love?", "Tell us about the best boss or mentor you ever had.", "Where do you hold back in this team, and why?"},
	},
	{ // Courage
		{"What's the most adventurous thing you've ever done, or eaten?", "Tell us about a time you were afraid and did it anyway.", "What fear still shapes the way you lead?"},
		{"What's a small brave thing you did this year?", "Tell us about a time you spoke up when staying quiet would have been easier.", "What risk do you know you need to take in the next year?"},
		{"What scares you that is completely harmless?", "Tell us about a time you started over.", "What would you do differently if you trusted yourself more?"},
		{"When did you last try something for the first time?", "Tell us about a decision others thought was crazy.", "What failure have you never fully talked about?"},
		{"What's the boldest thing on your bucket list?", "Tell us about someone whose courage inspired you.", "What do you need to let go of to lead better?"},
	},
	{ // Compassion
		{"Who was your favourite teacher, and why?", "Tell us about a time someone was kind to you when you didn't expect it.", "When did you last feel truly cared for, and by whom?"},
		{"What small thing instantly makes your day better?", "Tell us about a student, or a child, who changed you.", "What are you carrying this year that few people here know about?"},
		{"Who in your life always makes you laugh?", "Tell us about a time you were there for someone in a hard season.", "How do you take care of yourself when no one is watching?"},
		{"How do you most like to be appreciated?", "Tell us about a time you were wrong about someone.", "What kind of support do you find hard to receive?"},
		{"What act of kindness from your school days do you still remember?", "Tell us about a parent's or student's story that stayed with you.", "Who do you need to thank, or forgive, and haven't yet? (No names needed.)"},
	},
	{ // Commitment
		{"What habit or hobby have you kept for more than five years?", "Tell us about a promise you kept even though it cost you.", "Why are you still at ZERA? What keeps you here?"},
		{"What will you never give up (a food, a team, a ritual)?", "Tell us about the hardest season of your working life, and what got you through.", "What do you want to be true of ZERA when you're no longer here?"},
		{"What's the longest you've ever worked on one thing?", "Tell us about a time you almost quit.", "What are you committed to that few people see?"},
		{"What morning routine do you swear by?", "Tell us about the moment you knew education was your calling (or wasn't).", "What would make the next five years at ZERA worth it for you?"},
		{"Which family tradition do you keep?", "Tell us about someone who never gave up on you.", "What does faithfulness look like in your life right now?"},
	},
}

var HeartwoodCards = []string{
	"What do you want the people at this table to know about you that they might not?",
	"Who at this table has helped you grow, and how?",
	"What legacy do you want to leave in ZERA's forest?",
	"What would you want a student to say about you in 2035?",
	"What did today show you about this team?",
	"What promise would you make to this team for the year ahead?",
}

var BondCards = [3][]string{
	{"Find three things you have in common that have nothing to do with work.", "Swap the stories of how you each came to ZERA.", "Share your favourite childhood food and the memory behind it."},
	{"Each tell the story of a turning point in your life.", "What's a lesson you each learned the hard way?", "Who shaped you most before you turned 18?"},
	{"Tell your partner one thing you've learned about them today, and what it meant to you.", "What does each of you need from the other in the year ahead?", "Complete for each other: \"Working with you, I'd love more of ___.\""},
}

var SquirrelCards = []string{
	"What's the story behind that?",
	"How did that change you?",
	"What would ten-year-old you think of that?",
	"How does that show up in how you lead today?",
	"Who else was there, and what did they mean to you?",
	"What did you learn about yourself?",
}

var CampfireCards = []string{
	"One word for how you feel right now.",
	"Your ultimate comfort food.",
	"A song that always lifts you.",
	"The best advice you've ever received, in one line.",
	"Mountain, beach or city?",
	"A small win from this week.",
	"The best holiday you've ever had, in one sentence.",
	"Something you're looking forward to.",
}

// Power is one Enneagram type's gift. Each player picks their top three
// types and may activate each of those powers once per game.
type Power struct {
	Type  int    `json:"type"`
	Name  string `json:"name"`
	Gift  string `json:"gift"`
	Title string `json:"title"`
	When  string `json:"when"`
	Text  string `json:"text"`
}

var Powers = []Power{
	{1, "Reformer", "integrity", "True North", "your turn, before you start sharing", "Look at 3 cards from your region and choose the one to answer. +1 bonus point."},
	{2, "Helper", "care", "Open Hands", "your turn, before you start sharing", "Invite another player to answer your question too, after you. You both keep the card, and you both take 1 bonus point."},
	{3, "Achiever", "drive", "Momentum", "your turn, while moving", "Move up to 3 more spaces this turn."},
	{4, "Individualist", "depth", "Deep Water", "your turn, before you start sharing", "Answer the question one tier deeper than your ring (up to Oak). It still scores at your ring's tier."},
	{5, "Investigator", "insight", "Field Notes", "your turn, while moving", "See every hidden discovery next to you, then claim one without moving there."},
	{6, "Loyalist", "loyalty", "Rope Team", "your turn, while moving", "Jump to an ally's space from anywhere, without using your steps. You both take 1 bonus point."},
	{7, "Enthusiast", "joy", "Adventure", "your turn", "Call a Campfire: everyone answers a one-sentence question. +1 bonus point."},
	{8, "Challenger", "strength", "Champion", "after someone else's share", "Give that player 2 trust acorns at once, and take 1 bonus point."},
	{9, "Peacemaker", "harmony", "Common Ground", "at Dusk, before pairs are set", "Your alliance tonight works at any distance."},
}

// Discovery tokens: 26 for the 36 spaces around the Heartwood; the other 10
// spaces are quiet clearings with nothing to find.
var TokenMix = map[string]int{
	"mushroom": 9, "squirrel": 10, "campfire": 5, "path": 2,
}

var Colors = []string{"#e53935", "#fb8c00", "#fdd835", "#43a047", "#00897b", "#1e88e5", "#5e35b1", "#d81b60", "#6d4c41", "#546e7a"}
