/**
 * Convert page tab functionality
 */

document.addEventListener('DOMContentLoaded', function() {
  // Attach click handlers to all tab buttons
  const tabButtons = document.querySelectorAll('.tabs .tab[data-tab]');

  for (const button of tabButtons) {
    button.addEventListener('click', function() {
      const tabName = this.getAttribute('data-tab');
      showTab(tabName);
    });
  });
});
