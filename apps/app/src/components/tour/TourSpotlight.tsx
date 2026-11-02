// The layer under the bird's bubble (docs/specs/007-guided-tour.md,
// "Highlight"): it dims the page, cuts a hole around the target and swallows
// clicks, so the page and the tour stay in step.

const MARGIN = 8;

export function TourSpotlight({ rect }: { rect: DOMRect | null }) {
  return (
    <div
      className={`tour-layer${rect ? '' : ' no-target'}`}
      aria-hidden="true"
      onMouseDown={(event) => event.preventDefault()}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
      }}
    >
      {rect && (
        <div
          className="tour-hole"
          style={{
            top: rect.top - MARGIN,
            left: rect.left - MARGIN,
            width: rect.width + MARGIN * 2,
            height: rect.height + MARGIN * 2,
          }}
        />
      )}
    </div>
  );
}

export const SPOTLIGHT_MARGIN = MARGIN;
