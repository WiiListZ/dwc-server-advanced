export const MKW_COURSES = [
  "Mario Circuit", "Moo Moo Meadows", "Mushroom Gorge", "Grumble Volcano",
  "Toad's Factory", "Coconut Mall", "DK Summit", "Wario's Gold Mine",
  "Luigi Circuit", "Daisy Circuit", "Moonview Highway", "Maple Treeway",
  "Bowser's Castle", "Rainbow Road", "Dry Dry Ruins", "Koopa Cape",
  "GCN Peach Beach", "GCN Mario Circuit", "GCN Waluigi Stadium", "GCN DK Mountain",
  "DS Yoshi Falls", "DS Desert Hills", "DS Peach Gardens", "DS Delfino Square",
  "SNES Mario Circuit 3", "SNES Ghost Valley 2", "N64 Mario Raceway", "N64 Sherbet Land",
  "N64 Bowser's Castle", "N64 DK's Jungle Parkway", "GBA Bowser Castle 3", "GBA Shy Guy Beach",
]

export const MKW_REGIONS = ["Worldwide", "Japan", "United States", "Europe", "Australia", "Taiwan", "Korea", "China"]

export function courseName(id: number) {
  return id >= 0 && id < MKW_COURSES.length ? MKW_COURSES[id] : `Course ${id}`
}

export const regionName = (id: number) => MKW_REGIONS[id] || `Region ${id}`

export function formatScore(milliseconds: number) {
  const total = Number(milliseconds)
  if (!Number.isFinite(total) || total <= 0) return String(milliseconds)
  const minutes = Math.floor(total / 60000)
  const seconds = Math.floor((total % 60000) / 1000)
  return `${minutes}:${String(seconds).padStart(2, "0")}.${String(total % 1000).padStart(3, "0")}`
}

// Cups in game order; numbers are internal course IDs.
export const MKW_CUPS = [
  { name: "Mushroom", courses: [8, 1, 2, 4] },
  { name: "Flower", courses: [0, 5, 6, 7] },
  { name: "Star", courses: [9, 15, 11, 3] },
  { name: "Special", courses: [14, 10, 12, 13] },
  { name: "Shell", courses: [16, 20, 25, 26] },
  { name: "Banana", courses: [27, 31, 23, 18] },
  { name: "Leaf", courses: [21, 30, 29, 17] },
  { name: "Lightning", courses: [24, 22, 19, 28] },
]
