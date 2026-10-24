// Style sheets are bundled as text (build.mjs), for the in-page button's shadow root.
declare module "*.css" {
  const text: string;
  export default text;
}
