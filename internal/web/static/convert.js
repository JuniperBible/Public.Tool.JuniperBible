/**
 * Convert page tab functionality
 */

document.addEventListener('DOMContentLoaded', function() {
  // Attach click handlers to all tab buttons
  const tabButtons = document.querySelectorAll('.tabs .tab[data-tab]');

  tabButtons.forEach(function(button) {
    button.addEventListener('click', function() {
      const tabName = this.getAttribute('data-tab');
      showTab(tabName);
    });
  });
});
