export function SiteFooter() {
  return (
    <footer className="border-t border-border pt-6 pb-8">
      <p className="text-sm text-muted-foreground">
        Based on the NBCC calendar feed. Times are Saint John, NB (Atlantic Time). This is not an official NBCC source;
        confirm deadlines in Brightspace.
      </p>
      <p className="mt-2 text-sm text-muted-foreground">
        Icons: Solar by 480 Design, licensed under CC BY 4.0.
      </p>
      <p className="mt-2 text-sm">
        <a href="https://ewzxyh.com" className="text-primary underline underline-offset-4">
          Developed by Enzo Yoshida
        </a>
      </p>
    </footer>
  )
}
