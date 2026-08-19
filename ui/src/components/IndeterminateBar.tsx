export function IndeterminateBar() {
  return (
    <>
      <div className="h-1 w-full overflow-hidden bg-muted">
        <div className="h-full w-1/3 animate-[speedIndeterminate_1.2s_ease-in-out_infinite] bg-foreground/70" />
      </div>
      <style>{`
@keyframes speedIndeterminate {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(400%); }
}
`}</style>
    </>
  )
}
