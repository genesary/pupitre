import { Toaster as Sonner, type ToasterProps } from "sonner"

// sonner, unstyled and dressed with the pupitre toast classes (components.css:
// .toast, .toast-success…) and icons of layouts/App.astro, bottom right, 4 s.
const Toaster = (props: ToasterProps) => (
  <Sonner
    position="bottom-right"
    offset={24}
    duration={4000}
    icons={{
      success: <span>✓</span>,
      info: <span>ℹ️</span>,
      warning: <span>⚠</span>,
      error: <span>✕</span>,
    }}
    toastOptions={{
      unstyled: true,
      classNames: {
        toast: "toast",
        default: "toast-info",
        info: "toast-info",
        success: "toast-success",
        warning: "toast-warning",
        error: "toast-error",
      },
    }}
    {...props}
  />
)

export { Toaster }
