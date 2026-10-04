package main

// Card text and powers. The six values on the map are ZERA's core values,
// ZERAOS; each value's card has a Seed (light), Sapling (story) and Oak (deep)
// prompt.

// Value is one of ZERA's core values. The map's wedges follow them clockwise,
// so the forest spells Z-E-R-A-O-S. Players stand for one, and alliances join
// different ones.
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

var RegionNames = [6]string{"Zealous", "Excellence", "Resilience", "Authenticity", "Open-mindedness", "Sustainability"}

var RegionCards = [6][][3]string{
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
		{"What silly debate could you argue about forever? (Pineapple on pizza?)", "Tell us about a time a disagreement led to a better decision.", "How do you react when someone disagrees with you?"},                                               // Disagreeing well
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

// RegionThreads names each region card's theme, in RegionCards order (shown in the wiki).
var RegionThreads = [6][]string{
	{"What lights you up", "Cheering, not comparing", "Stepping up first", "In young people's corner", "Joining in wholeheartedly"},
	{"Beyond what was asked", "Doing it right when no one's looking", "Heart comes first", "Raising the bar together", "Getting better, kindly"},
	{"Stronger because of it", "When plans go wrong", "Hard words that helped", "Steady for others", "Not going it alone"},
	{"Belonging, not just fitting in", "Dropping the act", "Not having all the answers", "Where you come from", "Many ways to learn"},
	{"Changing your mind", "Learning from mistakes", "First impressions", "Disagreeing well", "Learning across differences"},
	{"Habits that last", "Things that run without you", "Planting for others", "The long game", "Starting with the end in mind"},
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

// StatementCard replaces the Bond card for a pair or trio forming an alliance:
// they find one line that holds all their values.
var StatementCard = "Find one line that holds all your values (a statement, a slogan or a cheer, about 12 words). Try: \"We ___ so that ___.\" One of you types it on your phone."

// StatementTip helps the Keeper when an alliance is stuck on its statement:
// what the values have in common, and example lines.
type StatementTip struct {
	Common   string   `json:"common"`
	Examples []string `json:"examples"`
}

// StatementTips is keyed by the alliance's value letters in ZERAOS order, e.g.
// "ZE", "ERS": 15 pairs and 20 trios.
var StatementTips = map[string]StatementTip{
	"ZE":  {"Heart first, then the high bar: passion is what lifts good work further.", []string{"We pour our hearts in so that good becomes extraordinary.", "Love the work first, then do it better than anyone asked.", "Heart first, bar high, every child, every day!"}},
	"ZR":  {"Passion that survives setbacks, and comes back stronger because of them.", []string{"We keep our spark through setbacks so that students learn how.", "A bad day is fuel, not a full stop.", "Knocked down? Fired up! Back again, stronger!"}},
	"ZA":  {"Speaking up for every child, with energy that is real, not for show.", []string{"We champion every child loudly so that no one feels left out.", "Cheer for every child, especially the one nobody else noticed.", "Loud and proud for every child!"}},
	"ZO":  {"Passion that still listens: strong convictions, held with an open hand.", []string{"We speak with fire and listen hard so that better ideas win.", "Care deeply, listen widely, change your mind gladly.", "Bring the passion, bring the questions!"}},
	"ZS":  {"A quick blaze or a steady fire? Passion paced to last for years.", []string{"We pace our passion so that it still burns in ten years.", "Light a fire that still glows long after we have gone.", "Steady flame, long game, keep it burning!"}},
	"ER":  {"Excellence isn't never falling; it's rising higher after every fall.", []string{"We learn hard from every miss so that next time is better.", "The bar rises every time we get back up.", "Miss it, learn it, raise it, nail it!"}},
	"EA":  {"High bars for everyone, without pretending everyone starts at the same line.", []string{"We set high hopes for every child so that all can shine.", "Every child's best, honestly measured, is our standard.", "All of us belong! All of us rise!"}},
	"EO":  {"Aim beyond expectation, yet treat each failure as a lesson, not a verdict.", []string{"We try bold ideas so that our best keeps getting better.", "Ask the question that makes good work great.", "Try it! Test it! Make it better!"}},
	"ES":  {"Brilliant today versus lasting for years: quality that still holds up later.", []string{"We build things properly so that they still work in ten years.", "Good enough for now is not good enough for later.", "Do it right, make it last!"}},
	"RA":  {"Strength that comes from honesty: owning the struggle is how we grow together.", []string{"We share our struggles honestly so that nobody struggles alone.", "Real people, real struggles, real growth. Everyone belongs here.", "Fall down, get up, together, as we are!"}},
	"RO":  {"Both see setbacks as teachers: not just bouncing back, but learning and changing.", []string{"We treat setbacks as lessons so that every failure leaves us wiser.", "Failure is feedback. Read it, then go again.", "Stumble, learn, grow, go!"}},
	"RS":  {"Built for the long haul: systems that get stronger each time they're tested.", []string{"We build systems that bend, not break, so that they outlast us.", "Plant deep roots; hard seasons only make them stronger.", "Bend, don't break! Grow back stronger!"}},
	"AO":  {"Every voice is real and welcome, and we're honestly changed by hearing it.", []string{"We listen to every voice so that every child sees themselves here.", "Come as you are; leave with a bigger view.", "Every voice in! Every mind open!"}},
	"AS":  {"Inclusion that outlasts any one leader: belonging built into how the school runs.", []string{"We build belonging into how we work so that it outlives us.", "Belonging should not depend on who is in charge this year.", "Everyone in, for good!"}},
	"OS":  {"Hold the end in mind firmly and the route loosely: plans that keep learning.", []string{"We keep asking new questions so that our plans age well.", "A good plan is one that can still learn.", "Eyes on the horizon, minds wide open!"}},
	"ZER": {"Heart, a high bar and staying power: aim high, fall, rise higher.", []string{"We aim high and get back up so that children learn how.", "The best work comes from people who care enough to try again.", "Heart in! Bar up! Never done!"}},
	"ZEA": {"Passion and high hopes for every child, not only the easy ones to celebrate.", []string{"We believe big for every child so that each one does too.", "Every child deserves someone who thinks they're extraordinary.", "Big hearts, high hopes, no one left out!"}},
	"ZEO": {"Ambitious and fired up, yet humble enough to learn from anyone in the room.", []string{"We chase better ideas from anywhere so that students get our best.", "Hungry to improve, happy to be wrong on the way.", "Dream big, ask questions, do better!"}},
	"ZES": {"Excellence today, passion for tomorrow: brilliant work done at a pace that lasts.", []string{"We do great work at a steady pace so that it lasts.", "Aim for excellence your successors will thank you for.", "Go hard, go well, go the distance!"}},
	"ZRA": {"Never giving up on any child, and being honest when it's hard.", []string{"We keep showing up for every child so that none feels forgotten.", "Some children take longer. We stay longer.", "No child too hard, no day too long!"}},
	"ZRO": {"Keen enough to try, tough enough to fail, open enough to change course.", []string{"We try bold things, then learn from flops, so that students dare.", "Try it with heart, learn from the wobble, then try again.", "Dream it! Try it! Flop it! Fix it!"}},
	"ZRS": {"Passion with stamina: energy that survives hard years with the long goal in sight.", []string{"We rest, recover and return with heart so that the mission lasts.", "Keep the fire lit through hard terms; the finish is years away.", "Still burning! Still standing! Still going!"}},
	"ZAO": {"Excited by everyone's ideas, so the quiet voices are heard, not just the loudest.", []string{"We get excited about every voice so that quiet ones speak up.", "The best idea in the room might come from the quietest person.", "All voices! All ideas! All in!"}},
	"ZAS": {"Passion for inclusion that's built to last, not a campaign that fades.", []string{"We build our care into habits so that every child belongs, always.", "Real inclusion isn't an event. It's how we do things here.", "All our heart, all the children, all the years!"}},
	"ZOS": {"Fired up for a long future, and willing to rethink how we get there.", []string{"We stay curious and keen so that the school keeps growing.", "Love the mission enough to keep rethinking how we get there.", "Fresh ideas! Big heart! Long view!"}},
	"ERA": {"High standards for every child, honest about the struggle, patient through it.", []string{"We admit what isn't working so that we can make it excellent.", "Honest about the struggle, ambitious about the outcome, for every child.", "Real effort! Real growth! Real results!"}},
	"ERO": {"Ambition that welcomes failure: the bar goes up because we're willing to be wrong.", []string{"We pilot, review and improve so that each year beats the last.", "Draft, critique, redraft: that's how great work is made.", "First try, try again, best try yet!"}},
	"ERS": {"Excellence that lasts: quality built to weather hard years, not just shine once.", []string{"We build strong foundations so that tough years make us better.", "Build it well, test it hard, leave it stronger.", "Stronger, better, year after year!"}},
	"EAO": {"Excellence looks different for each child; it takes an open mind to see it.", []string{"We redefine success child by child so that every gift is seen.", "There's more than one way to be brilliant.", "Many minds! Many gifts! All of them count!"}},
	"EAS": {"Quality and inclusion built into the bones of the school, not bolted on later.", []string{"We plan for every learner from day one so that all belong.", "Judge a school by how it serves every child, year after year.", "Built for all, built to last!"}},
	"EOS": {"Keep improving the system without tearing it up: steady progress, open to better.", []string{"We keep reviewing what works so that good systems keep getting better.", "Small improvements every term add up to a great school.", "Better each term! Wiser each year!"}},
	"RAO": {"A safe place to fail honestly: everyone belongs, mistakes included.", []string{"We own our mistakes openly so that children feel safe making theirs.", "Here, it's safe to be wrong, be yourself and try again.", "Messy, honest, learning, together!"}},
	"RAS": {"Belonging that holds through hard times: a community strong enough to last.", []string{"We hold together through hard seasons so that every child stays held.", "When it gets hard, nobody gets left behind, this year or next.", "Hard days? All of us, all the way!"}},
	"ROS": {"Adaptable for the long run: plans that learn and grow stronger from each setback.", []string{"We let every setback teach the system so that it gets wiser.", "Rethink, rebuild, and leave it sturdier for the next team.", "Learn it! Change it! Keep it going!"}},
	"AOS": {"Listening to every voice now, so tomorrow's school is shaped by all of us.", []string{"We let every voice shape decisions so that the future fits everyone.", "Plan with people, not for them, and it will last.", "Every voice today, a better school tomorrow!"}},
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
