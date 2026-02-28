package sword

// Vulgate versification system (Latin Vulgate / Catholic canon).
// Used by Catholic Bibles, includes deuterocanonical books.
// Key differences from KJV:
// - Includes 7 deuterocanonical books (Tobit, Judith, Wisdom, Sirach, Baruch, 1-2 Maccabees)
// - Different Psalm numbering (LXX/Vulgate Ps 9-147 offset from Hebrew/KJV)
// - Includes additions to Esther and Daniel
// - Daniel has 14 chapters (includes Greek additions)
//
// IMPORTANT: Book order matches SWORD's canon_vulg.h for correct verse indexing.
// Deuterocanonical books are interspersed with OT books, not at the end.
// Reference: https://github.com/bibletime/crosswire-sword-mirror/blob/master/include/canon_vulg.h

func init() {
	RegisterVersification(VulgSystem)
}

// VulgSystem is the Vulgate versification system.
// Book order matches SWORD's canon_vulg.h exactly for correct verse index calculation.
var VulgSystem = &VersificationSystem{
	Name: "Vulg",
	Books: []VersificationBook{
		// Old Testament with deuterocanonical books interspersed
		// Order follows SWORD's otbooks_vulg[] from canon_vulg.h
		{"Gen", "Genesis", "Gen", "OT", []int{31, 25, 24, 26, 31, 22, 24, 22, 29, 32, 32, 20, 18, 24, 21, 16, 27, 33, 38, 18, 34, 24, 20, 67, 34, 35, 46, 22, 35, 43, 55, 32, 20, 31, 29, 43, 36, 30, 23, 23, 57, 38, 34, 34, 28, 34, 31, 22, 32, 25}},
		{"Exod", "Exodus", "Exod", "OT", []int{22, 25, 22, 31, 23, 30, 25, 32, 35, 29, 10, 51, 22, 31, 27, 36, 16, 27, 25, 26, 36, 31, 33, 18, 40, 37, 21, 43, 46, 38, 18, 35, 23, 35, 35, 38, 29, 31, 43, 36}},
		{"Lev", "Leviticus", "Lev", "OT", []int{17, 16, 17, 35, 19, 30, 38, 36, 24, 20, 47, 8, 59, 57, 33, 34, 16, 30, 37, 27, 24, 33, 44, 23, 55, 45, 34}},
		{"Num", "Numbers", "Num", "OT", []int{54, 34, 51, 49, 31, 27, 89, 26, 23, 36, 34, 15, 34, 45, 41, 50, 13, 32, 22, 30, 35, 41, 30, 25, 18, 65, 23, 31, 39, 17, 54, 42, 56, 29, 34, 13}},
		{"Deut", "Deuteronomy", "Deut", "OT", []int{46, 37, 29, 49, 33, 25, 26, 20, 29, 22, 32, 32, 18, 29, 23, 22, 20, 22, 21, 20, 23, 30, 25, 22, 19, 19, 26, 68, 29, 20, 30, 52, 29, 12}},
		{"Josh", "Joshua", "Josh", "OT", []int{18, 24, 17, 25, 16, 27, 26, 35, 27, 43, 23, 24, 33, 15, 63, 10, 18, 28, 51, 9, 43, 34, 16, 33}},
		{"Judg", "Judges", "Judg", "OT", []int{36, 23, 31, 24, 32, 40, 25, 35, 57, 18, 40, 15, 25, 20, 20, 31, 13, 31, 30, 48, 24}},
		{"Ruth", "Ruth", "Ruth", "OT", []int{22, 23, 18, 22}},
		{"1Sam", "1 Samuel", "1Sam", "OT", []int{28, 36, 21, 22, 12, 21, 17, 22, 27, 27, 15, 25, 23, 52, 35, 23, 58, 30, 24, 43, 15, 23, 28, 23, 44, 25, 12, 25, 11, 31, 13}},
		{"2Sam", "2 Samuel", "2Sam", "OT", []int{27, 32, 39, 12, 25, 23, 29, 18, 13, 19, 27, 31, 39, 33, 37, 23, 29, 33, 43, 26, 22, 51, 39, 25}},
		{"1Kgs", "1 Kings", "1Kgs", "OT", []int{53, 46, 28, 34, 18, 38, 51, 66, 28, 29, 43, 33, 34, 31, 34, 34, 24, 46, 21, 43, 29, 54}},
		{"2Kgs", "2 Kings", "2Kgs", "OT", []int{18, 25, 27, 44, 27, 33, 20, 29, 37, 36, 21, 21, 25, 29, 38, 20, 41, 37, 37, 21, 26, 20, 37, 20, 30}},
		{"1Chr", "1 Chronicles", "1Chr", "OT", []int{54, 55, 24, 43, 26, 81, 40, 40, 44, 14, 46, 40, 14, 17, 29, 43, 27, 17, 19, 7, 30, 19, 32, 31, 31, 32, 34, 21, 30}},
		{"2Chr", "2 Chronicles", "2Chr", "OT", []int{17, 18, 17, 22, 14, 42, 22, 18, 31, 19, 23, 16, 22, 15, 19, 14, 19, 34, 11, 37, 20, 12, 21, 27, 28, 23, 9, 27, 36, 27, 21, 33, 25, 33, 27, 23}},
		{"Ezra", "Ezra", "Ezra", "OT", []int{11, 70, 13, 24, 17, 22, 28, 36, 15, 44}},
		{"Neh", "Nehemiah", "Neh", "OT", []int{11, 20, 31, 23, 19, 19, 73, 18, 38, 39, 36, 46, 31}},
		// Deuterocanonical: Tobit, Judith inserted after Nehemiah
		{"Tob", "Tobit", "Tob", "AP", []int{25, 23, 25, 23, 28, 22, 20, 24, 12, 13, 21, 22, 23, 17}},
		{"Jdt", "Judith", "Jdt", "AP", []int{12, 18, 15, 17, 29, 21, 25, 34, 19, 20, 21, 20, 31, 18, 15, 31}},
		{"Esth", "Esther", "Esth", "OT", []int{22, 23, 15, 17, 14, 14, 10, 17, 32, 13, 12, 6, 18, 19, 19, 24}},
		{"Job", "Job", "Job", "OT", []int{22, 13, 26, 21, 27, 30, 21, 22, 35, 22, 20, 25, 28, 22, 35, 23, 16, 21, 29, 29, 34, 30, 17, 25, 6, 14, 23, 28, 25, 31, 40, 22, 33, 37, 16, 33, 24, 41, 35, 28, 25, 16}},
		{"Ps", "Psalms", "Ps", "OT", []int{6, 13, 9, 10, 13, 11, 18, 10, 39, 8, 9, 6, 7, 5, 11, 15, 51, 15, 10, 14, 32, 6, 10, 22, 12, 14, 9, 11, 13, 25, 11, 22, 23, 28, 13, 40, 23, 14, 18, 14, 12, 6, 26, 18, 12, 10, 15, 21, 23, 21, 11, 7, 9, 24, 13, 12, 12, 18, 14, 9, 13, 12, 11, 14, 20, 8, 36, 37, 6, 24, 20, 28, 23, 11, 13, 21, 72, 13, 20, 17, 8, 19, 13, 14, 17, 7, 19, 53, 17, 16, 16, 5, 23, 11, 13, 12, 9, 9, 5, 8, 29, 22, 35, 45, 48, 43, 14, 31, 7, 10, 10, 9, 26, 9, 10, 2, 29, 176, 7, 8, 9, 4, 8, 5, 7, 5, 6, 8, 8, 3, 18, 3, 3, 21, 27, 9, 8, 24, 14, 10, 8, 12, 15, 21, 10, 11, 9, 14, 9, 6}},
		{"Prov", "Proverbs", "Prov", "OT", []int{33, 22, 35, 27, 23, 35, 27, 36, 18, 32, 31, 28, 25, 35, 33, 33, 28, 24, 29, 30, 31, 29, 35, 34, 28, 28, 27, 28, 27, 33, 31}},
		{"Eccl", "Ecclesiastes", "Eccl", "OT", []int{18, 26, 22, 17, 19, 11, 30, 17, 18, 20, 10, 14}},
		{"Song", "Song of Solomon", "Song", "OT", []int{16, 17, 11, 16, 17, 12, 13, 14}},
		// Deuterocanonical: Wisdom, Sirach inserted after Song
		{"Wis", "Wisdom", "Wis", "AP", []int{16, 25, 19, 20, 24, 27, 30, 21, 19, 21, 27, 27, 19, 31, 19, 29, 20, 25, 20}},
		{"Sir", "Sirach", "Sir", "AP", []int{40, 23, 34, 36, 18, 37, 40, 22, 25, 34, 36, 19, 32, 27, 22, 31, 31, 33, 28, 33, 31, 33, 38, 47, 36, 28, 33, 30, 35, 27, 42, 28, 33, 31, 26, 28, 34, 39, 41, 32, 28, 26, 37, 27, 31, 23, 31, 28, 19, 31, 38}},
		{"Isa", "Isaiah", "Isa", "OT", []int{31, 22, 26, 6, 30, 13, 25, 22, 21, 34, 16, 6, 22, 32, 9, 14, 14, 7, 25, 6, 17, 25, 18, 23, 12, 21, 13, 29, 24, 33, 9, 20, 24, 17, 10, 22, 38, 22, 8, 31, 29, 25, 28, 28, 26, 13, 15, 22, 26, 11, 23, 15, 12, 17, 13, 12, 21, 14, 21, 22, 11, 12, 19, 12, 25, 24}},
		{"Jer", "Jeremiah", "Jer", "OT", []int{19, 37, 25, 31, 31, 30, 34, 22, 26, 25, 23, 17, 27, 22, 21, 21, 27, 23, 15, 18, 14, 30, 40, 10, 38, 24, 22, 17, 32, 24, 40, 44, 26, 22, 19, 32, 20, 28, 18, 16, 18, 22, 13, 30, 5, 28, 7, 47, 39, 46, 64, 34}},
		{"Lam", "Lamentations", "Lam", "OT", []int{22, 22, 66, 22, 22}},
		// Deuterocanonical: Baruch inserted after Lamentations
		{"Bar", "Baruch", "Bar", "AP", []int{22, 35, 38, 37, 9, 72}},
		{"Ezek", "Ezekiel", "Ezek", "OT", []int{28, 9, 27, 17, 17, 14, 27, 18, 11, 22, 25, 28, 23, 23, 8, 63, 24, 32, 14, 49, 32, 31, 49, 27, 17, 21, 36, 26, 21, 26, 18, 32, 33, 31, 15, 38, 28, 23, 29, 49, 26, 20, 27, 31, 25, 24, 23, 35}},
		// Daniel with 14 chapters (includes Greek additions: Prayer of Azariah, Susanna, Bel)
		{"Dan", "Daniel", "Dan", "OT", []int{21, 49, 100, 34, 31, 28, 28, 27, 27, 21, 45, 13, 65, 42}},
		{"Hos", "Hosea", "Hos", "OT", []int{11, 24, 5, 19, 15, 11, 16, 14, 17, 15, 12, 14, 15, 10}},
		{"Joel", "Joel", "Joel", "OT", []int{20, 32, 21}},
		{"Amos", "Amos", "Amos", "OT", []int{15, 16, 15, 13, 27, 15, 17, 14, 15}},
		{"Obad", "Obadiah", "Obad", "OT", []int{21}},
		{"Jonah", "Jonah", "Jonah", "OT", []int{16, 11, 10, 11}},
		{"Mic", "Micah", "Mic", "OT", []int{16, 13, 12, 13, 14, 16, 20}},
		{"Nah", "Nahum", "Nah", "OT", []int{15, 13, 19}},
		{"Hab", "Habakkuk", "Hab", "OT", []int{17, 20, 19}},
		{"Zeph", "Zephaniah", "Zeph", "OT", []int{18, 15, 20}},
		{"Hag", "Haggai", "Hag", "OT", []int{14, 24}},
		{"Zech", "Zechariah", "Zech", "OT", []int{21, 13, 10, 14, 11, 15, 14, 23, 17, 12, 17, 14, 9, 21}},
		{"Mal", "Malachi", "Mal", "OT", []int{14, 17, 18, 6}},
		// Deuterocanonical: 1-2 Maccabees at end of OT
		{"1Macc", "1 Maccabees", "1Macc", "AP", []int{67, 70, 60, 61, 68, 63, 50, 32, 73, 89, 74, 54, 54, 49, 41, 24}},
		{"2Macc", "2 Maccabees", "2Macc", "AP", []int{36, 33, 40, 50, 27, 31, 42, 36, 29, 38, 38, 46, 26, 46, 40}},

		// New Testament (27 books - same as KJV)
		{"Matt", "Matthew", "Matt", "NT", []int{25, 23, 17, 25, 48, 34, 29, 34, 38, 42, 30, 50, 58, 36, 39, 28, 26, 35, 30, 34, 46, 46, 39, 51, 46, 75, 66, 20}},
		{"Mark", "Mark", "Mark", "NT", []int{45, 28, 35, 40, 43, 56, 37, 39, 49, 52, 33, 44, 37, 72, 47, 20}},
		{"Luke", "Luke", "Luke", "NT", []int{80, 52, 38, 44, 39, 49, 50, 56, 62, 42, 54, 59, 35, 35, 32, 31, 37, 43, 48, 47, 38, 71, 56, 53}},
		{"John", "John", "John", "NT", []int{51, 25, 36, 54, 47, 72, 53, 59, 41, 42, 57, 50, 38, 31, 27, 33, 26, 40, 42, 31, 25}},
		{"Acts", "Acts", "Acts", "NT", []int{26, 47, 26, 37, 42, 15, 59, 40, 43, 48, 30, 25, 52, 27, 41, 40, 34, 28, 40, 38, 40, 30, 35, 27, 27, 32, 44, 31}},
		{"Rom", "Romans", "Rom", "NT", []int{32, 29, 31, 25, 21, 23, 25, 39, 33, 21, 36, 21, 14, 23, 33, 27}},
		{"1Cor", "1 Corinthians", "1Cor", "NT", []int{31, 16, 23, 21, 13, 20, 40, 13, 27, 33, 34, 31, 13, 40, 58, 24}},
		{"2Cor", "2 Corinthians", "2Cor", "NT", []int{24, 17, 18, 18, 21, 18, 16, 24, 15, 18, 33, 21, 13}},
		{"Gal", "Galatians", "Gal", "NT", []int{24, 21, 29, 31, 26, 18}},
		{"Eph", "Ephesians", "Eph", "NT", []int{23, 22, 21, 32, 33, 24}},
		{"Phil", "Philippians", "Phil", "NT", []int{30, 30, 21, 23}},
		{"Col", "Colossians", "Col", "NT", []int{29, 23, 25, 18}},
		{"1Thess", "1 Thessalonians", "1Thess", "NT", []int{10, 20, 13, 18, 28}},
		{"2Thess", "2 Thessalonians", "2Thess", "NT", []int{12, 17, 18}},
		{"1Tim", "1 Timothy", "1Tim", "NT", []int{20, 15, 16, 16, 25, 21}},
		{"2Tim", "2 Timothy", "2Tim", "NT", []int{18, 26, 17, 22}},
		{"Titus", "Titus", "Titus", "NT", []int{16, 15, 15}},
		{"Phlm", "Philemon", "Phlm", "NT", []int{25}},
		{"Heb", "Hebrews", "Heb", "NT", []int{14, 18, 19, 16, 14, 20, 28, 13, 28, 39, 40, 29, 25}},
		{"Jas", "James", "Jas", "NT", []int{27, 26, 18, 17, 20}},
		{"1Pet", "1 Peter", "1Pet", "NT", []int{25, 25, 22, 19, 14}},
		{"2Pet", "2 Peter", "2Pet", "NT", []int{21, 22, 18}},
		{"1John", "1 John", "1John", "NT", []int{10, 29, 24, 21, 21}},
		{"2John", "2 John", "2John", "NT", []int{13}},
		{"3John", "3 John", "3John", "NT", []int{15}},
		{"Jude", "Jude", "Jude", "NT", []int{25}},
		{"Rev", "Revelation", "Rev", "NT", []int{20, 29, 22, 11, 14, 17, 17, 13, 21, 11, 19, 18, 18, 20, 8, 21, 18, 24, 21, 15, 27, 21}},

		// Additional Vulgate books (placed after NT in SWORD's canon_vulg.h)
		{"PrMan", "Prayer of Manasseh", "PrMan", "AP", []int{15}},
		{"1Esd", "1 Esdras", "1Esd", "AP", []int{58, 31, 24, 63, 73, 34, 15, 97, 56}},
		{"2Esd", "2 Esdras", "2Esd", "AP", []int{40, 48, 36, 52, 56, 59, 140, 63, 47, 60, 46, 51, 58, 48, 63, 78}},
		{"AddPs", "Psalm 151", "AddPs", "AP", []int{7}},
		{"EpLao", "Laodiceans", "EpLao", "AP", []int{20}},
	},
}
