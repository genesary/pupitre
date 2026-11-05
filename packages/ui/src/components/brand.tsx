/** The pupitre logo of the top bar: the pioupitre bird and "pupitre" in Caveat. */
export function Brand() {
  return (
    <>
      <img src="/pioupitre.png" alt="pioupitre" className="w-8 h-8 object-contain" />
      <span className="brand-font text-[1.3rem] font-bold text-ink">pupitre</span>
    </>
  );
}
