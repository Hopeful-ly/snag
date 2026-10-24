const delay = 450;

export const tooltip = $state<{ text: string; rect: DOMRect | null }>({ text: "", rect: null });

export function tip(node: HTMLElement, text: string) {
  let current = text;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const hide = () => {
    clearTimeout(timer);
    tooltip.rect = null;
  };
  const enter = () => {
    clearTimeout(timer);
    timer = setTimeout(() => {
      if (current) Object.assign(tooltip, { text: current, rect: node.getBoundingClientRect() });
    }, delay);
  };
  node.addEventListener("mouseenter", enter);
  node.addEventListener("mouseleave", hide);
  node.addEventListener("click", hide);
  return {
    update(text: string) {
      current = text;
      if (tooltip.rect) tooltip.text = text;
    },
    destroy() {
      hide();
      node.removeEventListener("mouseenter", enter);
      node.removeEventListener("mouseleave", hide);
      node.removeEventListener("click", hide);
    },
  };
}
