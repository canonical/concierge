import datetime
import os
import textwrap

project = "Concierge"
author = "Canonical Ltd."
copyright = f"{datetime.date.today().year}, {author}"
html_title = project + " documentation"

ogp_site_url = os.environ.get("READTHEDOCS_CANONICAL_URL", "/")
ogp_site_name = project
ogp_image = "https://assets.ubuntu.com/v1/cc828679-docs_illustration.svg"

html_context = {
    "product_page": "canonical.com/juju/docs",
    "product_tag": "_static/logos/juju-logo-no-text.png",
    "discourse": "https://discourse.charmhub.io",
    "mattermost": "",
    "matrix": "https://matrix.to/#/#charmhub-charmdev:ubuntu.com",
    "github_url": "https://github.com/canonical/concierge",
    "repo_default_branch": "main",
    "repo_folder": "/docs/",
    "display_contributors": False,
    "github_issues": "enabled",
    "author": author,
    "license": {
        "name": "Apache-2.0",
        "url": "https://github.com/canonical/concierge/blob/main/LICENSE",
    },
}

html_theme_options = {
    "source_edit_link": "https://github.com/canonical/concierge",
}

html_baseurl = os.environ.get("READTHEDOCS_CANONICAL_URL", "/")
sitemap_url_scheme = "{link}"
sitemap_show_lastmod = True
sitemap_excludes = ["404/", "genindex/", "search/"]

html_static_path = ["_static"]
templates_path = ["_templates"]

rediraffe_redirects = "redirects.txt"
rediraffe_dir_only = True

llms_txt_description = textwrap.dedent(
    """\
    This is the documentation for Concierge, an opinionated utility for provisioning
    charm development and testing machines.
    """
)

if os.environ.get("READTHEDOCS"):
    markdown_http_base = html_baseurl

linkcheck_ignore = [
    "http://127.0.0.1:8000",
    "https://github.com",
    r"https://matrix\.to/.*",
]
linkcheck_anchors_ignore_for_url = [r"https://github\.com/.*"]
linkcheck_retries = 3

myst_enable_extensions = {"colon_fence"}

extensions = [
    "canonical_sphinx",
    "notfound.extension",
    "sphinx_design",
    "sphinx_rerediraffe",
    "sphinx_reredirects",
    "sphinx_tabs.tabs",
    "sphinxcontrib.jquery",
    "sphinxext.opengraph",
    "sphinx_config_options",
    "sphinx_contributor_listing",
    "sphinx_filtered_toctree",
    "sphinx_llm.txt",
    "sphinx_related_links",
    "sphinx_roles",
    "sphinx_terminal",
    "sphinx_ubuntu_images",
    "sphinx_youtube_links",
    "sphinxcontrib.cairosvgconverter",
    "sphinx_last_updated_by_git",
    "sphinx.ext.intersphinx",
    "sphinx_sitemap",
]

exclude_patterns = [
    "doc-cheat-sheet*",
    ".venv*",
    "_dev",
    "_build",
    "README.md",
]

html_css_files = [
    "https://assets.ubuntu.com/v1/d86746ef-cookie_banner.css",
]

html_js_files = [
    "https://assets.ubuntu.com/v1/287a5e8f-bundle.js",
]

rst_prolog = """
.. role:: center
   :class: align-center
.. role:: h2
    :class: hclass2
.. role:: woke-ignore
    :class: woke-ignore
.. role:: vale-ignore
    :class: vale-ignore
"""

intersphinx_mapping = {
    "juju": ("https://canonical.com/juju/docs/juju-cli/latest/", None),
    "ops": ("https://canonical.com/juju/docs/ops/latest/", None),
    "charmcraft": ("https://canonical.com/juju/docs/charmcraft/4.3/", None),
}
