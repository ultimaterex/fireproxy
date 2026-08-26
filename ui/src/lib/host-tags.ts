export function hostTagsAdd(tags: string[], id: string): string[] {
  if (tags.includes(id)) return [...tags]
  return [...tags, id]
}

export function hostTagsRemove(tags: string[], id: string): string[] {
  return tags.filter((t) => t !== id)
}
