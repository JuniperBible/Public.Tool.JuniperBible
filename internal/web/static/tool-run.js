// Tool run page functionality

// Profile dropdown update
function updateProfiles() {
  const pluginSelect = document.getElementById('plugin');
  const profileSelect = document.getElementById('profile');
  const selectedOption = pluginSelect.options[pluginSelect.selectedIndex];
  const profilesData = selectedOption.getAttribute('data-profiles') || '';

  profileSelect.innerHTML = '<option value="">Select a profile...</option>';

  if (profilesData) {
    const profiles = profilesData.split(',');
    profiles.forEach(function(p) {
      const [id, desc] = p.split(':');
      if (id) {
        const option = document.createElement('option');
        option.value = id;
        option.textContent = id + (desc ? ' - ' + desc : '');
        profileSelect.appendChild(option);
      }
    });
  }
}

// Attach event listeners when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
  const pluginSelect = document.getElementById('plugin');
  if (pluginSelect) {
    pluginSelect.addEventListener('change', updateProfiles);
  }
});
